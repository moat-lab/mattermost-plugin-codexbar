# Architecture

## Runtime path

```text
Mattermost user
  -> /codexbar slash command
  -> Mattermost plugin server process
  -> rexec-go gRPC client
  -> Mac-side rexecd daemon
  -> /Applications/CodexBar.app/Contents/Helpers/CodexBarCLI
  -> JSON stdout
  -> rendered Mattermost bot cards
```

Mattermost is not the usage source of truth. The Mac-side CodexBar CLI is the source of truth; this plugin only validates command arguments, invokes a curated command subset, and renders user-friendly cards.

## Activation

On activation, the plugin:

1. Ensures the dedicated `codexbar` bot exists.
2. Resolves `CODEXBAR_REXECD_ADDR`.
3. Creates a `rexec-go` client.
4. Registers the `/codexbar` slash command.
5. Resolves the CodexBar binary and working directory.
6. Loads plugin display settings.

## Command lifecycle

For non-help commands:

1. The plugin checks that the command is running inside the CodexBar bot DM.
2. It builds a typed `codexbarRequest` from the slash-command tail.
3. It posts `Loading…`.
4. It returns immediately to Mattermost.
5. A background worker runs all required CodexBar invocations in parallel.
6. The worker deletes the loading post, posts rendered result cards, and clears older bot messages while preserving the new result.

## Safety boundaries

The plugin only exposes a curated read-oriented subset:

- usage/status
- local cost scan
- config validation
- help

It rejects arbitrary passthrough commands such as cache mutation.
