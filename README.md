# mattermost-plugin-codexbar

Mattermost server-side plugin that exposes CodexBar CLI data as private bot cards.

The plugin follows the `mattermost-plugin-fulcrum` shape:

- Mattermost creates a dedicated `codexbar` bot.
- `/codexbar` is registered as a plugin slash command.
- The plugin calls the operator Mac through `rexec-go`.
- The Mac-side `codexbar` CLI remains the source of truth.
- Mattermost receives curated, human-readable attachments instead of raw CLI passthrough.

## Commands

All commands are bot-DM only. Calls from public, private, or group channels return an ephemeral message telling the user to open the CodexBar bot DM.

| Command | CLI data source | Mattermost behavior |
|---|---|---|
| `/codexbar` | Fast Codex/Claude web usage probes plus `codexbar cost --format json --provider all` | Shows a low-latency live summary and local cost cards. |
| `/codexbar summary` | Same as `/codexbar` | Explicit summary alias. |
| `/codexbar usage [codex\|claude\|gemini\|all] [--source=auto\|web\|cli\|oauth\|api]` | `codexbar usage --format json --status`; provider `all` is split into bounded per-provider probes | Shows provider account, plan, rate windows, reset text, service status, and provider errors. |
| `/codexbar cost [codex\|claude\|gemini\|all] [--refresh]` | `codexbar cost --format json` | Shows local last-30-day and current-session token/cost cards. |
| `/codexbar config` | `codexbar config validate --format json` | Shows config validation health. |
| `/codexbar help` | local plugin help | Shows the curated Mattermost command surface. |

The plugin intentionally does not expose arbitrary `codexbar` subcommands such as cache mutation.

## Runtime Configuration

Set these environment variables on the Mattermost server process:

| Variable | Required | Meaning |
|---|---:|---|
| `CODEXBAR_REXECD_ADDR` | yes | gRPC address for the Mac-side rexecd daemon. |
| `CODEXBAR_BIN` | no | Binary path executed by rexecd. Defaults to `codexbar`; vctcn should use `/Applications/CodexBar.app/Contents/Helpers/CodexBarCLI`. |
| `CODEXBAR_CWD` | no | Working directory passed to rexecd. vctcn should use `/Applications/CodexBar.app/Contents/Helpers` so the Swift CLI resolves its app bundle correctly. |

The plugin has no Mattermost System Console settings. Deployment coordinates stay in the runtime environment/IaC.

## Build And Test

```bash
go test ./server/...
COPYFILE_DISABLE=1 make dist
```

`COPYFILE_DISABLE=1` is required on macOS so the plugin tarball does not contain `._*` AppleDouble files that break `mmctl --local plugin add`.

When exercising Mattermost slash commands through `/api/v4/commands/execute`,
include `team_id` even for the bot DM channel. Without it, Mattermost can return
`403 api.context.permissions.app_error` before the plugin sees the DM command.

The bundle is written to:

```text
dist/codexbar-<plugin.json version>.tar.gz
```

## Verified Local CLI Shape

On the operator Mac, CodexBar CLI 0.27.0 exposes the command surface this plugin uses:

```bash
/opt/homebrew/bin/codexbar usage --format json --pretty --status
/opt/homebrew/bin/codexbar cost --format json --pretty
/opt/homebrew/bin/codexbar config validate --format json
```

Observed provider JSON includes `codex`, `claude`, and `gemini` usage entries, plus local cost entries for providers with local token logs.
