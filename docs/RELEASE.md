# Publishing releases

Use the [Build and Publish workflow](../.github/workflows/publish.yml) to publish app images and GitHub releases.

| Trigger | Behavior |
| --- | --- |
| Scheduled, every 8 hours | Check upstream, update versions and pins, publish images and create a release. Skip when current. |
| Tag `vX.Y.Z` or `vX.Y.Z.W` | Build all apps using the tag's version and existing Beszel pins. |
| Manual run | Choose an app or `all`. Single-app rebuilds create no release. Use `force_rebuild` for an unchanged `all` build. |

## Manual build choices

| `image_type` | Directory |
| --- | --- |
| `all` | All apps |
| `stable` | `beszel_agent` |
| `smart` | `beszel_agent_smart` |
| `intel` | `beszel_agent_intel` |
| `amd` | `beszel_agent_amd` |
| `nvidia` | `beszel_agent_nvidia` |
| `hub` | `beszel_hub` |
| `bridge` | `beszel_bridge` |

When adding an app, update the workflow's build helpers and choices. Publish its images before advertising installation because unchanged versions can make scheduled runs skip it.

## Versions and architectures

The workflow uses `beszel_agent/config.yaml` as its app-version reference. Keep all app versions, image tags and the release tag aligned. The six Hub/Agent apps pin upstream binaries in `beszel_version`. Automatic releases update both versions. Tag releases keep existing binary pins.

The Bridge has no bundled Beszel binary. Its [compatibility policy](../beszel_bridge/compatibility.json) records tested Hub versions. New versions are accepted automatically, and the workflow tests the candidate Hub before publishing.

Images use one multi-architecture manifest without an `-{arch}` suffix. Intel is amd64-only. Other images support amd64/arm64. NVIDIA uses the Debian base. The workflow tests a local amd64 image before publishing.

## Bridge release checks

1. Complete the [HA OS checklist](MANUAL_CHECKLIST.md) for local and remote Hubs. Record tested versions, platforms and results in the release change.
2. Test the candidate Hub, including invalid-field isolation and recovery. Capture sanitized fixtures for new versions and record successful coverage in `tested_hub_versions`.
3. After validation, set `release_ready: true` in [compatibility.json](../beszel_bridge/compatibility.json) as part of the reviewed release change.
4. The workflow runs both Bridge test scripts against the candidate Hub version. Verify published image tags and installation on both architectures before announcing availability.

## Renaming an image

Publish the new image for every supported architecture, then bump the app version so Home Assistant detects the change. Keep the app slug unchanged to preserve options and data. Retain old image packages for existing installations that need repairs or rebuilds.
