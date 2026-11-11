**B1 PASS** — `.scratch/structured-logging/spec.md:87-149` now carries AD1–AD12 in full (AD2 60-slot map :91, AD5 table :97-107, AD6 :109, AD8 :113-119, AD9 :122, AD10 :124-146); no "unchanged from r6" placeholder remains (grep clean).

**B2 PASS (minor residual)** — `spec.md:114` states the exact groups: 11 shared + `error` (object+4) + `config` (object+21) + `resource` (object+3) + 25 retained + `backend_id`/`protocol_version` + 40 lane-only = **109**; `spec.md:14`/`:165` publish the dotted-path rule and the three object names, retract "17 config"/"26 retained". Cross-checked against AD2 raw map (`:91`): the 20 duplicate names are all literally present and `server` is genuinely absent from the 60 slots, so 60−20=40 and 11+5+22+4+25+2+40=109 is exact; 25 retained names count 25; 21 config subkeys = 10+6+5 as demanded. Residual: the full sorted 109-name list is derivable (60−20) but not printed verbatim — deterministic, low risk.

**B3 PASS** — `spec.md:178` retracts "compile-equivalent / NO shape change" and specifies `operatorLog` re-key to `config.*`/`error`; `spec.md:225` T4 declares "compile-preserving RE-KEY … NOT shape-preserving" and names the migrations: `manifest_test.go` 18-field assertions, `serve_edge_test.go` :211/:221 flat `"reason"`, `serve_test.go` :724-725.

**B4 PASS** — `spec.md:222` T1 adds `embedding_env_cli_test.go` (:49 plaintext fragments → JSON) and `embedding_env_process_test.go` (:65); `spec.md:227` T6 adds `main_test.go` :21-43; scope envelope `spec.md:236` tests 71→74; state effects include all three for T1 (`.omp/plans/execution-state.json:348-350`) and T6 (`:591-593`). Evidence gap: T1's verify pattern `-run 'CheckConfig|ExampleConfig'` may not exercise the two `embedding_env_*` tests (names unread); T6/T7 full-suite runs cover them later in the serial chain.

**B5 PASS** — state `AC04-8.ownerTaskIds = [T4,T5,T6,T7]` (`.omp/plans/execution-state.json:1046-1050`), and AC04-8 appears in T4/T5/T6/T7 `acceptanceIds` (`:534`, `:569`, `:603`, `:641`); spec `:202` stages evidence T4→T7 matching the ledger (`:271`).

**B6 PASS** — weights: `spec.md:28`/`:130`/`:139` assign `weight_invalid`/`weights_invalid` to `validateWeights` (bounds.go:16-61) via the node-returning `findScalar` wrapper (config.go:1075), explicitly NOT `validateBoundsAt`, with AD10 exception (d) for absent keys; server endpoint: `:22`/`:133`/`:170` assign BOTH `url_invalid` and `url_secret_parameter` (config.go:196-207) to the structure-stage root walk; embedding `url_invalid` (:450) stays under `resolveEmbeddingRef`/`resolveEmbeddingEnv` (`:24`/`:135`).

Limits: state section `sha256` values (`execution-state.json:11-25`) are not computable in this read-only context (no shell), so hash-to-spec binding is unverified; A1–A12 remain advisory (notably `spec.md:76` still describes state as `revision: r6` = A6; documentation drift, no safety consequence).

### Verdict
PASS