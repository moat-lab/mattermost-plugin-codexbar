# Deployment

## Build a bundle

```bash
go test ./server/...
COPYFILE_DISABLE=1 make dist
```

The bundle is written to:

```text
dist/codexbar-<plugin.json version>.tar.gz
```

`COPYFILE_DISABLE=1` is required on macOS so the tarball does not contain AppleDouble `._*` files that can break Mattermost plugin installation.

## Deploy with `pluginctl`

The repo includes a small REST deploy helper at `build/pluginctl/pluginctl`. It uses:

- `MM_SERVICESETTINGS_SITEURL`
- `MM_ADMIN_TOKEN`

Example:

```bash
MM_SERVICESETTINGS_SITEURL=https://mattermost.example.com \
MM_ADMIN_TOKEN=<admin-token> \
  ./build/pluginctl/pluginctl deploy codexbar dist/codexbar-<version>.tar.gz
```

`make deploy` runs the same helper after building the bundle.

## Deploy with `mmctl --local`

For local/container-side Mattermost administration, copy the bundle into the Mattermost container and install it with `mmctl --local`:

```bash
docker cp dist/codexbar-<version>.tar.gz \
  <mattermost-container>:/mattermost/data/codexbar-<version>.tar.gz

docker exec <mattermost-container> mmctl --local plugin disable codexbar || true
docker exec <mattermost-container> mmctl --local plugin delete codexbar || true
docker exec <mattermost-container> mmctl --local plugin add /mattermost/data/codexbar-<version>.tar.gz --force
docker exec <mattermost-container> mmctl --local plugin enable codexbar
```

## Verify deployed version

```bash
docker exec <mattermost-container> mmctl --local plugin list | grep -i codexbar
```

Expected shape:

```text
codexbar: CodexBar, Version: <version>
```

## IaC-managed deployment note

In the vctcn deployment, the Mattermost plugin is version-pinned by IaC and installed through the `apps/vctcn-app1` plugin install script. Direct `mmctl` installation is useful for emergency validation, but durable version changes should be reflected in the owning IaC release/version path.
