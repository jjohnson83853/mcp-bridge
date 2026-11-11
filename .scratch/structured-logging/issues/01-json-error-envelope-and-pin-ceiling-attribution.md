# 01 - JSON Error Envelope and Pin-Ceiling Attribution

Status: ready-for-agent
Blocked-by: none

## What to build
A rejected configuration must be self-explanatory without reading source. When the pins list exceeds its ceiling, the process emits exactly one newline-delimited JSON object on stderr and exits 1. The record names the first rejected entry, its document position, and the safe identity of that entry, so an operator can open the file at that line and delete the right pin.

The envelope is the contract every later ticket extends:

```json
{
  "schema_version": 1,
  "level": "error",
  "service": "mcp-bridge",
  "event": "config_validation_failed",
  "message": "configuration contains more pins than supported",
  "operation": "load_config",
  "outcome": "failure",
  "error": {
    "code": "pin_ceiling",
    "kind": "configuration",
    "retryable": false,
    "remediation": "remove pins at index 64 or later"
  },
  "config": {
    "path": "/app/configs/config.yaml",
    "property_path": "pins[64]",
    "line": 83,
    "column": 5,
    "pin_server": "alpha",
    "pin_tool": "tool-65",
    "actual_count": 65,
    "max_count": 64,
    "first_excess_index": 64
  }
}
```

Rules this ticket establishes: no application timestamp; no `environment`, `version`, or `subsystem`; no plaintext prefix or wrapper; no raw YAML, URL, or Go error text; one event per failure, never a detail-plus-summary pair; line and column are one-based.

## Acceptance Criteria
- [ ] AC-1: A document with 65 pins produces exactly one JSON line on stderr and exit code 1.
- [ ] AC-2: The record carries `property_path: "pins[64]"`, the one-based line and column of that sequence element, `actual_count: 65`, `max_count: 64`, and `first_excess_index: 64`.
- [ ] AC-3: The record carries the safe `pin_server` and `pin_tool` of the first excess entry, and no other pin's values.
- [ ] AC-4: The record contains no `time`, `timestamp`, `environment`, `version`, `subsystem`, or plaintext `mcp-bridge:` prefix.
- [ ] AC-5: A document with exactly 64 pins still starts normally and emits no error record.
- [ ] AC-6: Tests assert the emitted line parses as a single JSON object and that stdout stays empty on this failure.
