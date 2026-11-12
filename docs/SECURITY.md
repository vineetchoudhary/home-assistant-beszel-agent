# Security

Only the latest release receives security fixes.

Beszel Bridge requests `homeassistant_api` access to enable its existing integration-disabled entities when `enable_all_entities` is turned on. This permission grants administrator API access through Supervisor. The Bridge uses it only to read the entity registry and clear the integration-disabled flag on its selected MQTT entities. It preserves explicit user/device disables and does not log the Supervisor token. With the option off, it does not call the Core API.

Report app vulnerabilities privately through a [security advisory](https://github.com/vineetchoudhary/home-assistant-beszel-agent/security/advisories) or [email](mailto:vineet@developerinsider.co). Avoid public issues.

Report upstream Beszel or Agent vulnerabilities to the [Beszel maintainers](https://github.com/henrygd/beszel/security).
