# Security

Report beszel and beszel-agent security issues to [beszel maintainers](https://github.com/henrygd/beszel/security).

## Which Versions Get Updates?

Only the latest version gets security fixes. If you're behind, update to get the latest patches.

| Version | Supported          |
| ------- | ------------------ |
| Latest  | :white_check_mark: |
| Older   | :x:                |

## Verifying the Agent Download

Every app downloads its Beszel binary from the upstream GitHub release at build time. Each app directory carries a `beszel_sha256` file holding the SHA-256 of the exact release assets it installs, copied from the `beszel_<version>_checksums.txt` that upstream publishes with each release.

The Dockerfile looks up the asset it is about to install and verifies the tarball before unpacking it. A digest that does not match, or an asset with no pinned digest at all, fails the build rather than installing an unverified binary. TLS alone is no longer the only thing standing between a build and a tampered artifact.

The digests are refreshed by `scripts/refresh-checksums.sh`, which both `scripts/bump-version.sh --beszel` and the publish workflow call whenever `beszel_version` moves.

## Found a Security Issue in app?

**Please don't open a public issue.** That just tips off the bad actors.

Instead open a [secuirity advisory](https://github.com/vineetchoudhary/home-assistant-beszel-agent/security/advisories) or contact [me](mailto:vineet@developerinsider.co) directly.
