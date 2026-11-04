# DEV-RESULT v1

DEV-RESULT v1 is an opt-in result contract for foreground repository validation. It replaces unbounded terminal output with a deterministic summary while retaining complete redacted stdout and stderr for later inspection.

## Activation and compatibility

DEV-RESULT is enabled per command in `.dev-config.yaml`:

```yaml
dev_result:
  commands:
    test:
      enabled: true
      diagnostics_adapter:
        command: [node, tools/dev-tools-diagnostics.mjs]
        start_when: [junit/**/*.xml, turbo-summary.json]
    lint:
      enabled: true
      failure:
        lines: 120
        bytes: 16384
  retention:
    max_age: 168h
    max_bytes: 1073741824
```

Supported names are `test`, `test:focused`, `build`, `lint`, `lint:dead-code`, and `typecheck`. A command is unchanged when its entry is absent, unlisted, or has `enabled: false`. Watch, daemon, and background execution also retain their existing behavior.

There is no pipe mode and no second validation runner. `dev-tools result` only reads artifacts created by an enabled command.

## Repository diagnostics adapter

`test` and `test:focused` may configure one diagnostics adapter. For every enabled foreground validation, dev-tools creates a unique private run directory and adds `DEV_TOOLS_RUN_ID` and `DEV_TOOLS_RUN_DIR` only to that command invocation. The validation command keeps its existing selection, coverage, cache, timing, retry, and exit-code behavior. The adapter starts after validation only when a `start_when` glob matches a file created inside that run directory.

`command` is an argument vector, not a shell command. Dev-tools executes it directly from the project directory, writes exactly one JSON request to standard input, and accepts exactly one JSON response from standard output. Shell syntax inside an argument is passed literally.

The version 1 request is:

```json
{
  "schema": "dev-tools/diagnostics-request/v1",
  "run_id": "test-016",
  "run_dir": "/project/.build/agent/runs/test-016",
  "command": "test",
  "exit_code": 17,
  "duration_ms": 842,
  "timed_out": false,
  "cancelled": false
}
```

The version 1 response is:

```json
{
  "schema": "dev-tools/diagnostics-response/v1",
  "evidence_state": "partial",
  "failures": [
    {
      "identity": "billing/annual-statement",
      "message": "exports the annual statement failed",
      "package": "billing",
      "file": "statement.go",
      "line": 73,
      "suite": "annual statement",
      "test": "exports"
    }
  ],
  "warnings": [],
  "problems": [{"message": "billing.xml is missing"}],
  "sources": [{"kind": "junit", "path": "junit/billing.xml"}]
}
```

`evidence_state` is `complete`, `partial`, or `unavailable`. Every failure and warning has a stable `identity` and a non-empty `message`; location and test metadata are optional. Source paths are relative to the current run directory.

Default adapter limits are a 2-second runtime, 1 MiB stdout, 64 KiB stderr, 1,000 diagnostic records, 10 MiB per source, and 50 MiB across sources. They may be overridden with `timeout`, `stdout_bytes`, `stderr_bytes`, `max_diagnostics`, `max_source_bytes`, and `max_total_source_bytes` under `diagnostics_adapter`.

Complete evidence replaces rendered-log failures. Partial evidence keeps valid structured diagnostics and always emits a visible partial-evidence warning, alongside any reported evidence problems. Missing, incompatible, malformed, timed-out, cancelled, unsafe, or failed adapters produce a visible warning and use the bounded rendered-log fallback. A failed primary command with no actionable structured failure also falls back. In every case the primary validation outcome and exit code remain authoritative.

## Text envelope

The first line identifies the schema version and final outcome:

```text
DEV-RESULT v1 FAIL
command: dev-tools build
exit: 42 | duration: 3.41s | attempts: 1/1
summary: Validation failed with 9 distinct failure(s) and 2 warning(s).
failures: shown 2/9
- internal/parser.go:84: expected identifier before closing brace
- cmd/root.go:312: result artifact was not written
warnings: shown 1/2
- warning: package cache could not be reused
artifacts: log=.build/agent/logs/build-20261103T142501.000000000Z.log | result=.build/agent/results/build-20261103T142501.000000000Z.json
drill-down: full=dev-tools result build-20261103T142501.000000000Z --full | failures=dev-tools result build-20261103T142501.000000000Z --failures | warnings=dev-tools result build-20261103T142501.000000000Z --warnings
truncation: true | lines=12/80 | bytes=733/12288 | failures=2/9 | warnings=1/2
```

Failure and warning sections are omitted when their totals are zero. Required metadata, artifact paths, drill-down commands, and truncation counts remain visible. Evidence is normalized by removing ANSI sequences, progress updates, repeated lines, and other non-actionable noise before applying the budget.

## JSON sidecar

The sidecar uses `schema: dev-result/v1` and contains:

- a collision-safe job identifier;
- the outcome and normalized command;
- exit code, duration, and attempt counts;
- summary, distinct failures, and warnings;
- repository-relative artifact paths and retrieval commands;
- selected/total evidence and line/byte budget information;
- redaction state and match count.

`--format=json` emits the compact sidecar-shaped result and never embeds raw stdout or stderr. The retained sidecar contains structured evidence used by `--failures` and `--warnings`.

## Retained log

The private log artifact separates the complete streams without claiming byte-exact interleaving:

```text
DEV-RESULT-LOG v1
job: build-20261103T142501.000000000Z
command: dev-tools build

--- stdout ---
<complete redacted stdout>
--- stderr ---
<complete redacted stderr>
```

Logs and sidecars are written atomically with mode `0600` below `.build/agent/logs/` and `.build/agent/results/`. Valid adapter responses and source copies are stored below `.build/agent/evidence/<job>/`; `--full` reads those retained copies along with stdout and stderr.

## Evidence security and retention

Dev-tools rejects absolute and parent-traversal source paths, non-regular files, and symlinks that resolve outside the current run directory. Adapter output and every accepted source are bounded and redacted before durable persistence. The raw run scratch directory is removed after evidence is processed.

Retention defaults to seven days and 1 GiB. Cleanup removes the oldest unleased jobs beyond either limit, protects every live validation process independently of completed-artifact age, and eventually removes scratch/evidence orphaned by a terminated process. Active build reports can protect jobs with `.build/agent/leases/<lease-id>.json`:

```json
{
  "schema": "dev-tools/retention-lease/v1",
  "owner": "build-42",
  "run_ids": ["test-016"],
  "expires_at": "2026-11-04T15:00:00Z"
}
```

Expired leases are ignored. If any lease is malformed, cleanup fails closed, deletes nothing, and emits a warning in the current result.

## Default budgets

| Outcome | Lines | Bytes |
|---|---:|---:|
| Pass | 10 | 1,024 |
| Failure | 80 | 12,288 |
| Timeout or cancellation | 30 | 4,096 |
| Background start (model only; integration is deferred) | 3 | 1,024 |

Each enabled command may override any outcome budget with `lines` of at least 7 and `bytes` of at least 1,024. Those minima keep the mandatory envelope metadata representable. Selection is deterministic: required metadata comes first, followed by the first distinct actionable failures, then warnings. Counts always state how much evidence is shown versus retained.

## Redaction

Redaction happens before artifact persistence and compact rendering. DEV-RESULT replaces these values with `[REDACTED]`:

- authorization credentials and bearer tokens;
- private-key blocks;
- values assigned to common token, secret, password, credential, private-key, and API-key names;
- nontrivial values of secret-named environment variables.

The result records only whether redaction occurred and the number of matches. It does not retain original values or reversible digests.

## Progressive disclosure

```bash
# Re-render the compact result
dev-tools result <job>

# Read the complete retained streams
dev-tools result <job> --full

# Read all structured failures or warnings
dev-tools result <job> --failures
dev-tools result <job> --warnings
```

All four forms are read-only and never invoke the configured validation command.
