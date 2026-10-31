# Releases

The [publish workflow](../.github/workflows/publish.yml) supports these paths:

| Trigger | Behavior |
| --- | --- |
| Scheduled, every 8 hours | Check upstream, update app versions/binary pins, publish images, commit changes and create a tag/release. Skip when already current. |
| Tag `vX.Y.Z` or `vX.Y.Z.W` | Build all apps with the tag's app version and existing Beszel pins. |
| Manual **Build and Publish** | Choose an app or `all`. Single-app rebuilds run even at the same version and create no tag/release. Use `force_rebuild` for an unchanged `all` build. |

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
| `bridge` | `beszel_bridge`, subject to its release gate |

Add new apps to the workflow's build-helper table and dispatch choices. Publish their images before advertising installation: unchanged versions can make scheduled runs skip them.

## Version numbers

All apps share the version from `beszel_agent/config.yaml`. Their `config.yaml`, image tags and release tag must agree. The six bundled apps pin upstream binaries in `beszel_version`. Automatic releases update both versions, while tag releases retain those binary pins.

The Bridge has no bundled Beszel binary or `beszel_version` file. Its [compatibility policy](../beszel_bridge/compatibility.json) separately declares supported Hub schemas.

## Bridge release gate

Full and Bridge-only publishing are blocked by `release_ready: false` before version changes or registry login. Individual existing-app rebuilds remain available.

1. Complete and record the [manual HA OS checks](MANUAL_CHECKLIST.md) and [readiness results](BRIDGE_READINESS.md) for both Hub locations.
2. For new Hub schemas, capture sanitized fixtures and verify units, source timestamps and real HA entity behavior before updating compatibility.
3. Set `release_ready: true` in the reviewed release change. Keep `stage: experimental` during preview.
4. Publishing still runs both Bridge test scripts against the candidate Hub version. Unsupported schemas fail. Verify both architecture image tags and public pull/install before claiming availability.

## Images and Architectures

Each app publishes one image manifest without an `-{arch}` suffix. Intel is amd64-only, NVIDIA uses Debian, and other supported images cover amd64/arm64. Smoke tests load a single amd64 image locally before publication. See [local build commands](TESTING.md#building-a-multi-arch-image-locally).

### Migrating an image name

Publish the new image for every supported architecture, then bump the app version so Supervisor notices the change. Slugs preserve options/data. Keep old image packages available for installations that may repair or rebuild before updating.
