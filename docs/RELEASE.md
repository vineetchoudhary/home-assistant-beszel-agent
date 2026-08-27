# Releases

Three ways a release happens.

## Automatic (Every 8 Hours)
To keep the add-ons synced with upstream Beszel automatically, the GitHub Actions Workflow checks for new Beszel versions and:

1. Checks upstream Beszel for a new release
2. Updates `config.yaml`, `beszel_version`, and `CHANGELOG.md` for every add-on
3. Builds and publishes images to GHCR
4. Commits the repository changes
5. Creates a git tag and GitHub release if needed

This only does anything when upstream has actually moved. If `beszel_agent/config.yaml` already matches the resolved version, the run stops early and nothing is built.

## Manual Releases (With GitHub Tag)
Release add-on improvements:

1. Make code changes
2. Commit and push
3. Create a new git tag like `vX.Y.Z` or `vX.Y.Z.W`
4. Watch GitHub Actions publish the matching add-on version

A tag build always builds **all** add-ons, using the add-on version from the tag and the upstream Beszel version already pinned in `beszel_version`.

## Manual Builds (workflow_dispatch)
Run **Build and Publish** from the Actions tab to rebuild one add-on without waiting for upstream:

| `image_type` | Builds |
| --- | --- |
| `all` | every add-on (skipped if the version is already up-to-date, unless `force_rebuild`) |
| `hub` | `beszel_hub` |
| `hub-dev` | `beszel_hub_dev` |
| `stable` | `beszel_agent` |
| `dev` | `beszel_agent_dev` |
| `smart` | `beszel_agent_smart` |

Naming a single add-on is treated as an explicit request, so it builds even when the version is unchanged. Single-add-on builds do **not** create a git tag or GitHub release - only `all` does.

This is the path to use when adding a new add-on: its images do not exist in GHCR yet, and the scheduled run will skip while the version already matches upstream. Dispatch the new add-on's `image_type` once so Home Assistant has an image to pull.

## Version Numbers
Two different versions to keep track of:

- **Beszel version**: stored in `beszel_version`
- **Add-on version**: stored in `config.yaml` and git tags

Tag-based releases use the git tag as the add-on version and keep the current upstream Beszel version from `beszel_version`.

Automatic upstream releases update both values to the new upstream version.

All add-ons share one version number, driven by `beszel_agent/config.yaml`. Keep them in step - a `config.yaml` version with no matching image tag in GHCR means Home Assistant cannot install that add-on.
