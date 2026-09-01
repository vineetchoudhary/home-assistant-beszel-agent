# Releases

Three ways a release happens.

## Automatic (Every 8 Hours)
To keep the apps synced with upstream Beszel automatically, the GitHub Actions Workflow checks for new Beszel versions and:

1. Checks upstream Beszel for a new release
2. Updates `config.yaml`, `beszel_version`, and `CHANGELOG.md` for every app
3. Builds and publishes images to GHCR
4. Commits the repository changes
5. Creates a git tag and GitHub release if needed

This only does anything when upstream has actually moved. If `beszel_agent/config.yaml` already matches the resolved version, the run stops early and nothing is built.

## Manual Releases (With GitHub Tag)
Release app improvements:

1. Make code changes
2. Commit and push
3. Create a new git tag like `vX.Y.Z` or `vX.Y.Z.W`
4. Watch GitHub Actions publish the matching app version

A tag build always builds **all** apps, using the app version from the tag and the upstream Beszel version already pinned in `beszel_version`.

## Manual Builds (workflow_dispatch)
Run **Build and Publish** from the Actions tab to rebuild one app without waiting for upstream:

| `image_type` | Builds |
| --- | --- |
| `all` | every app (skipped if the version is already up-to-date, unless `force_rebuild`) |
| `stable` | `beszel_agent` |
| `smart` | `beszel_agent_smart` |
| `intel` | `beszel_agent_intel` |
| `amd` | `beszel_agent_amd` |
| `nvidia` | `beszel_agent_nvidia` |
| `hub` | `beszel_hub` |

The list lives in one place - the app table in the `Prepare build helpers` step of `publish.yml`. Adding an app means adding a row there (`id`, directory, image suffix, platforms, base, smoke-test kind) and a matching `image_type` choice in the `workflow_dispatch` inputs.

Naming a single app is treated as an explicit request, so it builds even when the version is unchanged. Single-app builds do **not** create a git tag or GitHub release - only `all` does.

This is the path to use when adding a new app: its images do not exist in GHCR yet, and the scheduled run will skip while the version already matches upstream. Dispatch the new app's `image_type` once so Home Assistant has an image to pull.

## Version Numbers
Two different versions to keep track of:

- **Beszel version**: stored in `beszel_version`
- **App version**: stored in `config.yaml` and git tags

Tag-based releases use the git tag as the app version and keep the current upstream Beszel version from `beszel_version`.

Automatic upstream releases update both values to the new upstream version.

All apps share one version number, driven by `beszel_agent/config.yaml`. Keep them in step - a `config.yaml` version with no matching image tag in GHCR means Home Assistant cannot install that app.

## Images and Architectures

Each app publishes **one multi-arch image**, built by a single
`docker buildx build --platform linux/amd64,linux/arm64` per app. There is no
`-{arch}` suffix; `config.yaml` names the image directly and Home Assistant
resolves the right architecture at pull time.

Smoke tests build a single `linux/amd64` image with `--load`, so it stays in the
runner's local Docker store and an image that fails its test never reaches the
registry.

`beszel_agent_intel` is amd64-only (`intel_gpu_top` is not packaged for
aarch64) and `beszel_agent_nvidia` builds on the Debian base. Both facts live in
the app table, not in per-app workflow steps.

### Migrating an image name

Renaming an app's image - as the move off `-{arch}` did - is safe, but it only
reaches installed apps on a **version bump**:

- `AppManager.update()` refuses to run when the installed version already equals
  the store version, so a rename alone is invisible to existing installs.
- On a real update, `App.update()` records the old image, pulls the new one, and
  then cleans up the old image itself. App data and options are keyed by slug,
  so nothing is lost.

Practical consequence: publish the new image name for **every** supported
architecture before the version bump lands, and keep the old `-{arch}` packages
around afterwards - an app that has not updated yet still resolves the old
name from its stored data if it is repaired or rebuilt.
