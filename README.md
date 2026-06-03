# mattermost-plugin-codexbar

Mattermost server-side plugin that exposes CodexBar CLI data as private bot cards.

The plugin follows the `mattermost-plugin-fulcrum` shape:

- Mattermost creates a dedicated `codexbar` bot.
- `/codexbar` is registered as a plugin slash command.
- The plugin calls the operator Mac through `rexec-go`.
- The Mac-side `CodexBarCLI` remains the source of truth.
- Mattermost receives curated, human-readable attachments instead of raw CLI passthrough.

## Documentation

The detailed documentation is split by category:

| Category | Document | Use when you need to... |
|---|---|---|
| User commands | [`docs/commands.md`](docs/commands.md) | Run `/codexbar`, understand bot-DM behavior, or choose provider/source arguments. |
| Architecture | [`docs/architecture.md`](docs/architecture.md) | Understand Mattermost → plugin → rexec → Mac-side CodexBarCLI flow. |
| Runtime configuration | [`docs/runtime-configuration.md`](docs/runtime-configuration.md) | Configure Mattermost environment variables and plugin settings. |
| Deployment | [`docs/deployment.md`](docs/deployment.md) | Build, package, install, upgrade, or verify the Mattermost plugin bundle. |
| Development and tests | [`docs/development.md`](docs/development.md) | Work on the Go code, run tests, and inspect command construction. |
| Troubleshooting | [`docs/troubleshooting.md`](docs/troubleshooting.md) | Diagnose slow commands, wrong plan detection, macOS permission prompts, or rexec failures. |

## Quick start

```bash
go test ./server/...
COPYFILE_DISABLE=1 make dist
```

The bundle is written to `dist/codexbar-<plugin.json version>.tar.gz`.

`COPYFILE_DISABLE=1` is required on macOS so the plugin tarball does not contain `._*` AppleDouble files that break `mmctl --local plugin add`.
