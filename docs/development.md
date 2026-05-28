# Development and tests

## Repository layout

| Path | Purpose |
|---|---|
| `server/` | Mattermost plugin server implementation. |
| `server/command.go` | Slash-command parsing, provider/source selection, rexec invocation, async response lifecycle. |
| `server/render.go` | JSON decoding and Mattermost attachment rendering. |
| `server/configuration.go` | Runtime environment and plugin setting resolution. |
| `build/pluginctl/` | Minimal REST deploy helper. |
| `plugin.json` | Mattermost plugin manifest and display settings schema. |
| `Makefile` | Cross-platform plugin binary build and tarball packaging. |

## Test commands

```bash
go test ./server/...
go test -count=1 ./server/...
COPYFILE_DISABLE=1 make dist
```

Use `-count=1` when you need to prove the current checkout rather than cached test results.

## Command construction tests

The command tests cover:

- `/codexbar` and `/codexbar summary` ordering.
- provider split for `/codexbar usage all`.
- default provider sources (`codex=oauth`, `claude=web`, `gemini=api`).
- explicit `--source cli|oauth|api` parsing.
- rejection of unsupported passthrough commands.
- parallel invocation ordering.

When changing provider defaults, update tests first or alongside the implementation so the intended source selection is explicit.

## Runtime verification expectation

A passing build is not enough for changes that affect command behavior. The strongest practical verification is:

1. Run `go test -count=1 ./server/...`.
2. Build with `COPYFILE_DISABLE=1 make dist`.
3. Install the bundle into Mattermost.
4. Execute `/codexbar summary` in the CodexBar bot DM.
5. Confirm rendered Source/Plan fields and absence of `DeadlineExceeded` errors.
