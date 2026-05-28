# User commands

All commands are private bot-DM commands. Calls from public, private, or group channels return an ephemeral message telling the user to open the CodexBar bot DM.

## Command surface

| Command | CLI data source | Mattermost behavior |
|---|---|---|
| `/codexbar` | Codex OAuth usage, fast Claude web usage, plus local cost scan | Shows a live usage summary and local cost cards. |
| `/codexbar summary` | Same as `/codexbar` | Explicit summary alias. |
| `/codexbar usage [codex\|claude\|gemini\|all] [--source=cli\|oauth\|api]` | `CodexBarCLI usage --format json --status` | Shows provider account, plan, rate windows, reset text, service status, and provider errors. |
| `/codexbar cost [codex\|claude\|gemini\|all] [--refresh]` | `CodexBarCLI cost --format json` | Shows local last-30-day and current-session token/cost cards. |
| `/codexbar config` | `CodexBarCLI config validate --format json` | Shows CodexBar config validation health. |
| `/codexbar help` | Plugin-local help text | Shows the curated Mattermost command surface. |

The plugin intentionally does not expose arbitrary CodexBar subcommands such as cache mutation.

## Default provider sources

Provider defaults are selected for correctness and latency:

| Provider | Default source | Reason |
|---|---|---|
| `codex` | `oauth` | The CLI source can report the account as `free`; OAuth reports the correct Pro plan. |
| `claude` | `web` | Faster than Claude CLI status in this deployment. The plugin adds `--web-timeout 20`. |
| `gemini` | `api` | Gemini status comes from the OAuth/API path. |

`/codexbar usage all` splits provider probes into bounded per-provider invocations instead of asking CodexBar to do one monolithic all-provider call.

## Response timing

The plugin posts a `Loading…` bot message first and returns from the slash-command handler immediately. Provider probes continue in a background worker. When the worker finishes, the plugin deletes the loading post and creates the final card post.

This avoids Mattermost slash-command timeouts when provider probes take longer than Mattermost allows a synchronous command handler to block.
