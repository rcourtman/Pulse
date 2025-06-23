## 🎉 Version 3.31.1

This patch release includes important bug fixes to improve the user experience.

---

## What's Changed

### 🐛 Bug Fixes
- **Reduced noisy logging** - Drastically reduced spammy log output by removing unnecessary debug messages, Content-Type header logs, and repetitive status logs. Logs are now cleaner and focused on actual errors/warnings (#198)
- **Fixed timezone handling in backup summary** - Corrected date display in backup summary cards for users in negative timezone offsets. The summary card now correctly shows the same date that was clicked in the calendar (#120)

---

## 📥 Installation & Updates

See the [Updating Pulse](https://github.com/rcourtman/Pulse#-updating-pulse) section in README for detailed update instructions.

### 🐳 Docker Images
- `docker pull rcourtman/pulse:v3.31.1`
- `docker pull rcourtman/pulse:latest` (stable releases)