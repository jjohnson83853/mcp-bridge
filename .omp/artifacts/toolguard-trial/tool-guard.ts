// Project-scoped Tool Guard trial adapter (NOT YET INSTALLED ACTIVE).
//
// Maps OMP `tool_call` events onto Tool Guard's Claude-shaped PreToolUse
// payload, runs `tg hook`, and enforces its decision. This ADDS a runtime tool
// firewall; it does not replace or disable the SpecDD/dispatch gates in
// .omp/RULES.md or workflow-policy/.
//
// Decision map:
//   allow -> run
//   deny  -> block
//   ask   -> main session with UI: operator confirm; otherwise block
// Internal errors (missing binary/policy, timeout, malformed output, spawn
// failure) block consequential tools (bash/write/edit/eval) and allow
// read-only tools, mirroring the shipped guard-hook.sh.

import type { ExtensionAPI } from "@oh-my-pi/pi-coding-agent";
import { spawnSync } from "node:child_process";
import { mkdirSync } from "node:fs";
import { join, resolve } from "node:path";

const PROJECT_ROOT = "/home/opencode/projects/default/mcp-bridge";
const STATE_ROOT = "/home/opencode/.local/share/tool-guard/mcp-bridge";
const TG_BIN = process.env.OMP_TOOLGUARD_TG ?? join(STATE_ROOT, "bin", "tg");
const CONSEQUENTIAL: Record<string, true> = { bash: true, write: true, edit: true, eval: true };
const READ_ONLY: Record<string, true> = { read: true, grep: true, glob: true, find: true, lsp: true };
const WRITE_TOOLS: Record<string, true> = {
	write: true,
	edit: true,
	notebookedit: true,
	apply_patch: true,
	multiedit: true,
	ast_edit: true,
};

type Guarded = { toolName: string; input: Record<string, unknown> };

// OMP tool names -> the names/groups Tool Guard knows. Unknown names keep
// their name and land in its `shell` group, which is the conservative
// fallback: no policy can silently treat them as a harmless read.
function canonicalize(toolName: string, input: Record<string, unknown>): Guarded {
	if (toolName === "eval") {
		// eval() is an unmetered shell/JS surface: sh(), Bun.$, node:fs.
		// Without this the guard has a one-call bypass. Treat the code text as
		// a shell command so deny-class shell rules still see it.
		const code = typeof input.code === "string" ? input.code : "";
		return { toolName: "bash", input: { ...input, command: code } };
	}
	if (toolName === "ast_edit") return { toolName: "edit", input };
	if (READ_ONLY[toolName]) return { toolName: "read", input };
	return { toolName, input };
}

function evaluate(guarded: Guarded, cwd: string, policyDir: string, hookFile: string) {
	const auditDir = join(STATE_ROOT, "audit");
	mkdirSync(auditDir, { recursive: true });
	const args = [
		"hook",
		"-policy-dir", policyDir,
		"-mode", "enforcement",
		"-dialect", "claude",
		"-escalate-as", "ask",
		"-fail-closed-tools", "bash,write,edit",
		"-fail-closed-groups", "shell,filesystem_writes",
		"-protect-path", policyDir,
		"-protect-path", hookFile,
		"-protect-path", STATE_ROOT,
		"-audit-log", join(auditDir, `hook-audit-${new Date().toISOString().slice(0, 10)}.jsonl`),
	];
	const payload = JSON.stringify({ tool_name: guarded.toolName, tool_input: guarded.input, cwd });
	const res = spawnSync(TG_BIN, args, {
		input: payload,
		encoding: "utf8",
		timeout: 5000,
		maxBuffer: 1 << 20,
	});
	if (res.error || res.status !== 0 || typeof res.stdout !== "string" || res.stdout.trim() === "") {
		return { decision: "error", reason: res.error?.message ?? `tg hook exited ${res.status}` } as const;
	}
	try {
		const doc = JSON.parse(res.stdout) as {
			hookSpecificOutput?: { permissionDecision?: string; permissionDecisionReason?: string };
		};
		const decision = doc.hookSpecificOutput?.permissionDecision;
		const reason = doc.hookSpecificOutput?.permissionDecisionReason ?? "Tool Guard denied the call";
		if (decision === "allow" || decision === "deny" || decision === "ask") return { decision, reason } as const;
		return { decision: "error", reason: "tg hook returned no usable decision" } as const;
	} catch {
		return { decision: "error", reason: "tg hook returned malformed JSON" } as const;
	}
}

export default function toolGuard(pi: ExtensionAPI): void {
	pi.on("tool_call", async (event, ctx) => {
		const root = ctx.cwd;
		// Project-pinned: a stray copy of this file governs nothing elsewhere.
		if (resolve(root) !== PROJECT_ROOT) return;
		const policyDir = join(STATE_ROOT, "policies");
		const hookFile = resolve(root, ".omp", "hooks", "pre", "tool-guard.ts");

		const guarded = canonicalize(event.toolName, event.input as Record<string, unknown>);
		const verdict = evaluate(guarded, root, policyDir, hookFile);

		if (verdict.decision === "error") {
			if (guarded.toolName === "bash" || WRITE_TOOLS[guarded.toolName]) {
				return { block: true, reason: `Tool Guard failed closed for ${guarded.toolName}: ${verdict.reason}` };
			}
			return;
		}
		if (verdict.decision === "deny") {
			return { block: true, reason: `Tool Guard: ${verdict.reason}` };
		}
		if (verdict.decision === "ask") {
			if (ctx.agent.kind === "main" && ctx.hasUI) {
				const ok = await ctx.ui.confirm("Tool Guard approval", `${guarded.toolName}: ${verdict.reason}`);
				if (ok) return;
				return { block: true, reason: `Tool Guard: operator declined (${verdict.reason})` };
			}
			return { block: true, reason: `Tool Guard: escalation needs an operator with UI (${verdict.reason})` };
		}
		return;
	});
}
