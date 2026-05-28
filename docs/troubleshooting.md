# Troubleshooting

## Mattermost command times out

Symptom: Mattermost reports slash-command timeout before cards appear.

Expected current behavior: the plugin should post `Loading…`, return immediately, then replace loading with result cards from a background worker.

Checks:

1. Verify the deployed plugin includes the async command lifecycle and is the expected version:
   ```bash
   docker exec <mattermost-container> mmctl --local plugin list | grep -i codexbar
   ```
2. Inspect Mattermost logs for plugin activation and result-post errors:
   ```bash
   docker logs --since 10m <mattermost-container> 2>&1 | grep -i codexbar
   ```
3. Test Mac-side rexecd reachability from the Mattermost host:
   ```bash
   getent hosts macmini.mouriya.lan
   timeout 5 bash -lc 'cat < /dev/null > /dev/tcp/macmini.mouriya.lan/50052'
   ```

## Codex shows Free plan instead of Pro

Symptom: Codex card shows `Plan=free`.

Cause: the Codex CLI source can report the login method as free even when the OAuth source reports Pro.

Expected default: Codex usage should run with `--source oauth`.

Verification on the operator Mac:

```bash
/Applications/CodexBar.app/Contents/Helpers/CodexBarCLI usage --format json --status --provider codex --source oauth
```

The rendered Mattermost card should show `Source=oauth` and `Plan=pro`.

## Claude is slow

Symptom: Claude usage takes tens of seconds.

Cause: Claude CLI status can be much slower than the web source in this deployment.

Expected default: Claude usage should run with `--source web --web-timeout 20`.

Verification on the operator Mac:

```bash
/Applications/CodexBar.app/Contents/Helpers/CodexBarCLI usage --format json --status --provider claude --source web --web-timeout 20
```

## macOS asks for permissions or password

The plugin itself runs in Mattermost, but macOS prompts are triggered by the Mac-side execution chain:

```text
rexecd -> CodexBarCLI -> CodexBar app/browser/session data
```

Common causes:

- The active rexecd binary path changed, so macOS treats it as a new client.
- `CodexBar.app` was updated and privacy/Gatekeeper state changed.
- The probe touches browser/session data or protected locations.
- The app or helper still has quarantine attributes.

Mitigations:

- Keep `CODEXBAR_BIN` fixed to `/Applications/CodexBar.app/Contents/Helpers/CodexBarCLI`.
- Keep rexecd on one stable path rather than alternating between build-tree and `~/.local/bin` paths.
- Prefer non-browser sources where correct (`codex=oauth`, `gemini=api`).
- Grant permissions to the actual binaries macOS reports in the prompt.

## rexec `DeadlineExceeded`

Symptom: a card says `rexec: run: rpc error: code = DeadlineExceeded`.

Checks:

1. Confirm the provider command is not exceeding its plugin timeout.
2. Run the exact CodexBarCLI command locally on the Mac.
3. Confirm TCP connectivity from Mattermost host to the rexecd address.
4. Check whether rexecd is still running on the Mac.

The plugin runs provider probes in parallel; one provider can fail while other provider/cost cards render successfully.
