# Publishing releases

Use the [Build and Publish workflow](../.github/workflows/publish.yml) to publish app images and GitHub releases.

| Trigger | Behavior |
| --- | --- |
| Scheduled, every 8 hours | Check upstream, update versions and pins, publish images and create a release. Skip when current. |
| Tag `vX.Y.Z` or `vX.Y.Z.W` | Build all apps using the tag's version and existing Beszel pins. |
| Manual run | Choose `all`, `all-except-bridge`, or one app. Set `bridge_version` with `bridge` to publish an independent Bridge update. |

## Manual build choices

| `image_type` | Directory |
| --- | --- |
| `all` | All apps |
| `all-except-bridge` | All Agent variants and Hub; Bridge is excluded |
| `stable` | `beszel_agent` |
| `smart` | `beszel_agent_smart` |
| `intel` | `beszel_agent_intel` |
| `amd` | `beszel_agent_amd` |
| `nvidia` | `beszel_agent_nvidia` |
| `hub` | `beszel_hub` |
| `bridge` | `beszel_bridge` |

These choices and version overrides are available only on **Run workflow**. Select a branch for a manual run so the workflow can commit its version updates. Scheduled checks and pushed `v*` tags always select all apps.

To release an upstream update while leaving Bridge untouched, select `all-except-bridge`. The workflow updates, tests and publishes only the six Agent/Hub apps, then creates the normal `v<version>` tag and GitHub release. It skips Bridge compatibility checks and preserves Bridge's configuration and changelog. If upstream is already current, enable `force_rebuild` to rebuild those six images; forced full builds create no new GitHub release.

To publish only a Bridge update, select `bridge` and enter a newer `bridge_version`, for example `0.21.1.4`. The workflow tests Bridge against the repository-pinned Hub version, updates only Bridge, and publishes its images and a `bridge-v<version>` GitHub release. This release does not replace the repository-wide **Latest** release or trigger another full build. Agent/Hub versions and upstream pins stay in place. Leave `bridge_version` blank to rebuild Bridge at its existing version without creating a new release. Versions use three or four numeric components; an older Bridge version is rejected. A newer explicit Bridge version creates its release even when `force_rebuild` is enabled.

Other single-app choices rebuild the selected app at the current shared Agent/Hub version and create no GitHub release. `bridge_version` is accepted only with `image_type=bridge`.

When adding an app, update the workflow's app table and choices and the release planner's `IMAGE_TYPES`. Publish its images before advertising installation because unchanged versions can make scheduled runs skip it.

## Versions and architectures

The workflow uses `beszel_agent/config.yaml` as the shared Agent/Hub version reference. Those six apps also pin upstream binaries in `beszel_version`. Automatic releases update both versions. Tag releases keep existing binary pins. Bridge can carry its own app version after a scoped release. Every image's version must match its own app's `config.yaml`; full builds preserve an independently published Bridge version when it is newer than the shared version.

The Bridge has no bundled Beszel binary. Its [compatibility policy](../beszel_bridge/compatibility.json) records tested Hub versions. New versions are accepted automatically, and the workflow tests the candidate Hub before publishing.

Images use one multi-architecture manifest without an `-{arch}` suffix. Intel is amd64-only. Other images support amd64/arm64. NVIDIA uses the Debian base. The workflow tests a local amd64 image before publishing.

## Bridge release checks

Bridge and full-repository releases run both Bridge test scripts against the candidate Hub version before publishing. There is no manual readiness flag. The Bridge uses the stable app stage.

1. The workflow checks unit tests, captured fixtures, builds and real Hub, MQTT and Home Assistant behavior, including invalid-field isolation and recovery.
2. Capture sanitized fixtures for new Hub versions and record successful coverage in `tested_hub_versions` in [compatibility.json](../beszel_bridge/compatibility.json).
3. Use the [HA OS checklist](MANUAL_CHECKLIST.md) to record Supervisor installation and physical platform acceptance separately. This checklist does not disable publishing.
4. Verify published image tags and installation on both architectures before announcing availability.

## Renaming an image

Publish the new image for every supported architecture, then bump the app version so Home Assistant detects the change. Keep the app slug unchanged to preserve options and data. Retain old image packages for existing installations that need repairs or rebuilds.
