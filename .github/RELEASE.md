# Releases
Two types of releases:

## Automatic (Every 8 Hours)
To keep the add-on synced with upstream Beszel automatically, the GitHub Actions Workflow checks for new Beszel versions and:

1. Updates config.yaml
2. Builds all architectures
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

- **Beszel version** (in config.yaml): `0.17.0` - the upstream agent
- **Add-on version** (repo tag): `v1.0.0` -  repository release

Tag-based releases always use the latest Beszel version. The config version is updated during the build.