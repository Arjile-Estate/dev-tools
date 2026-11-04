# DEV-RESULT v1

DEV-RESULT v1 is an opt-in result contract for foreground repository validation. It replaces unbounded terminal output with a deterministic summary while retaining complete redacted stdout and stderr for later inspection.

## Activation and compatibility

DEV-RESULT is enabled per command in `.dev-config.yaml`:

```yaml
dev_result:
  commands:
    test:
      enabled: true
    lint:
      enabled: true
      failure:
        lines: 120
        bytes: 16384
```

Supported names are `test`, `test:focused`, `build`, `lint`, `lint:dead-code`, and `typecheck`. A command is unchanged when its entry is absent, unlisted, or has `enabled: false`. Watch, daemon, and background execution also retain their existing behavior.

There is no pipe mode and no second validation runner. `dev-tools result` only reads artifacts created by an enabled command.

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

Logs and sidecars are written atomically with mode `0600` below `.build/agent/logs/` and `.build/agent/results/`.

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
