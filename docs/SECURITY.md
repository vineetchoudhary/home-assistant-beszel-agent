# Security

Only the latest release receives security fixes.

Beszel Bridge requests `homeassistant_api` access for optional entity enabling and history retention. This permission grants administrator API access through Supervisor. With `enable_all_entities` enabled, it reads the entity registry and clears the integration-disabled flag on its selected MQTT entities, preserving explicit user/device disables. With `history_retention_days` above `0`, it requests Recorder cleanup using only exact current IDs of its selected entities. It never uses wildcard or domain cleanup, changes the global Recorder policy, or logs the Supervisor token. With both options off, it does not call the Core API.

Report app vulnerabilities privately through a [security advisory](https://github.com/vineetchoudhary/home-assistant-beszel-agent/security/advisories) or [email](mailto:vineet@developerinsider.co). Avoid public issues.

Report upstream Beszel or Agent vulnerabilities to the [Beszel maintainers](https://github.com/henrygd/beszel/security).
