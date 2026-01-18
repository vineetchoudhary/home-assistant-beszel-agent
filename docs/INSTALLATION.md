# Beszel Agent Add-on Installation & Setup Guide

This guide walks you through installing and configuring the Beszel Agent add-on for Home Assistant, using step-by-step screenshots.

---

## 1. Open Home Assistant Add-ons
Open your Home Assistant instance and navigate to Settings. Click on "Add-ons":
![Step 1](screenshots/1-ha-settings.webp)

---

## 2. Open Add-ons Store
Click on "Add-on Store" button on bottom right:
![Step 2](screenshots/2-ha-add-ons.webp)

---

## 3. Open Add-ons Store Repositories
Click on the three dots in the top right and select "Repositories":
![Step 3](screenshots/3-ha-add-ons-store-repo.webp)

---

## 4. Add Custom Repository
Paste following URL and click "Add" button:
```
https://github.com/vineetchoudhary/home-assistant-beszel-agent
```

![Step 4](screenshots/4-ha-add-ons-add-repo.webp)

---

## 5. Confirm Repository Added
You should see the Beszel Agent repository listed:
![Step 5](screenshots/5-ha-add-ons-repo-added.webp)

---

## 6. Install Beszel Agent Add-on
Click on the "Beszel Agent" add-on and then click "Install":
![Step 6](screenshots/6-ha-beszel-agenet-install.webp)

---

## 7. Open Add-on Configuration
After installation, open the configuration tab:
![Step 7](screenshots/7-ha-beszel-agenet-config.webp)

---

## 8. Fill in Required Configuration
Enter your SSH key, Hub URL, and Token:

![Step 8](screenshots/8-ha-beszel-agenet-config-fill.webp)

---

## 9. (Optional) Configure Custom Volumes
Add custom volume mappings if needed:
![Step 9](screenshots/9-ha-beszel-agenet-custom-volumes.webp)

---

## 10. Start the Add-on
Navigate back to the "Info" tab and click "Start":
![Step 10](screenshots/10-ha-beszel-agenet-install-success.webp)

---

## 11. Observe Add-on Running
You should see the add-on running successfully (You can check the logs for connection status).
![Step 11](screenshots/11-ha-beszel-agenet-start.png.webp)

---

## 12. (Optional) Disable Protection Mode
If you are not seeing expected metrics, try disabling protection mode. This is mostly required for other Add-ons stats (docker stats) monitoring.
![Step 12](screenshots/12-ha-beszel-agenet-protection-mode.png.webp)

---

## Troubleshooting & Support
- If you encounter issues, check the add-on logs for errors.
- For advanced configuration, see the main documentation or open an issue on GitHub.

---

**Enjoy monitoring your Home Assistant system with Beszel Agent!**
