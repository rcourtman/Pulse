# Changelog for v3.22.1

## ✨ Features
- [fddc9c2](https://github.com/rcourtman/pulse/commit/fddc9c2) feat: implement sudoless update system with polkit integration

## 🔄 All Changes
- [fddc9c2](https://github.com/rcourtman/pulse/commit/fddc9c2) feat: implement sudoless update system with polkit integration

### Key Improvements
- **Sudoless Updates**: No more manual sudo commands required for updates
- **Polkit Integration**: Secure privilege escalation for service restarts  
- **Enhanced Update Process**: Multi-strategy restart approach with better reliability
- **Comprehensive Testing**: New test suite covering update workflows
- **Improved UI**: Better progress tracking and user feedback during updates
- **Automatic Setup**: Install script now automatically configures polkit rules

### Technical Details
- Added polkit rule for service management without sudo
- Fixed npm dependency conflicts during update extraction
- Implemented graceful fallback strategies for service restart
- Enhanced frontend with new 'restarting' phase indicator
- Resolved race conditions in update completion flow

This release significantly improves the update experience by eliminating the need for manual intervention during the update process.