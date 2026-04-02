# Releases
Two types of releases:

## Automatic (Every 8 Hours)
To keep the add-on synced with upstream Beszel automatically, the GitHub Actions Workflow checks for new Beszel versions and:

1. Updates config.yaml
2. Builds the supported 64-bit architectures (`amd64`, `aarch64`)
3. Pushes to GHCR
4. Commits the config.yaml change


## Manual Releases (With GitHub Tag)
Release add-on improvements:

1. Make code changes
2. Commit and push
3. Create new git tag (vX.Y.Z)
4. Watch GitHub Actions build

## Version Numbers
Two different versions to keep track of:

- **Beszel version** (in `beszel_version`): `0.18.6` - the upstream agent
- **Add-on version** (in `config.yaml` and repo tag): `0.18.6.1` / `v0.18.6.1` - repository release

Tag-based releases use the git tag as the add-on version and keep the current upstream Beszel version from `beszel_version`.
