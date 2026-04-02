# Releases
Two types of releases:

## Automatic (Every 8 Hours)
To keep the add-on synced with upstream Beszel automatically, the GitHub Actions Workflow checks for new Beszel versions and:

1. Checks upstream Beszel for a new release
2. Updates `config.yaml`, `beszel_version`, and `CHANGELOG.md`
3. Builds and publishes images to GHCR
4. Commits the repository changes
5. Creates a git tag and GitHub release if needed


## Manual Releases (With GitHub Tag)
Release add-on improvements:

1. Make code changes
2. Commit and push
3. Create a new git tag like `vX.Y.Z` or `vX.Y.Z.W`
4. Watch GitHub Actions publish the matching add-on version

## Version Numbers
Two different versions to keep track of:

- **Beszel version**: stored in `beszel_version`
- **Add-on version**: stored in `config.yaml` and git tags

Tag-based releases use the git tag as the add-on version and keep the current upstream Beszel version from `beszel_version`.

Automatic upstream releases update both values to the new upstream version.
