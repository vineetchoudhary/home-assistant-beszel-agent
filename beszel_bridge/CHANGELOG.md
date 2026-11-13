# Changelog

## 0.21.2

- Verify compatibility with Beszel Hub 0.21.2.
- Fix false release compatibility failures on hosts with temperature sensors, with separate checks for absent sensors and valid zero readings.
- Add optional `group_entities`, disabled by default, to combine SMART disk, container and systemd service details into attributes on one existing status/state entity per component.
- Preserve summary entity IDs, exact SMART raw values and source freshness when switching between grouped and individual entities.
