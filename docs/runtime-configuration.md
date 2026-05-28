# Runtime configuration

## Mattermost process environment

Set these environment variables on the Mattermost server process:

| Variable | Required | Default | Meaning |
|---|---:|---|---|
| `CODEXBAR_REXECD_ADDR` | yes | none | gRPC address for the Mac-side rexecd daemon. |
| `CODEXBAR_BIN` | no | `/Applications/CodexBar.app/Contents/Helpers/CodexBarCLI` | Binary path executed by rexecd. |
| `CODEXBAR_CWD` | no | `/Applications/CodexBar.app/Contents/Helpers` | Working directory passed to rexecd so the Swift CLI resolves its app bundle correctly. |

Deployment coordinates stay in runtime environment/IaC, not Mattermost System Console settings.

## Mattermost plugin settings

The Mattermost System Console exposes business/display settings only:

| Setting | Default | Meaning |
|---|---:|---|
| `HideAccountValues` | `true` | When enabled, usage and summary cards render Account fields as `***` without changing CodexBar CLI execution or output. |

## Mac-side rexecd expectations

The plugin expects the configured rexecd endpoint to be able to execute:

```bash
/Applications/CodexBar.app/Contents/Helpers/CodexBarCLI usage --format json --status --provider codex --source oauth
/Applications/CodexBar.app/Contents/Helpers/CodexBarCLI usage --format json --status --provider claude --source web --web-timeout 20
/Applications/CodexBar.app/Contents/Helpers/CodexBarCLI usage --format json --status --provider gemini --source api
/Applications/CodexBar.app/Contents/Helpers/CodexBarCLI cost --format json --provider all
/Applications/CodexBar.app/Contents/Helpers/CodexBarCLI config validate --format json
```

For the operator Mac deployment, the intended fixed binary path is `/Applications/CodexBar.app/Contents/Helpers/CodexBarCLI`; avoid switching between symlinks and build-tree paths because macOS privacy permissions are path/signature sensitive.
