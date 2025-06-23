## v3.31.0-rc2 Release Candidate

This release candidate includes significant improvements to the release process and workflow automation.

### 🚀 Release Process Improvements

- **Automated Draft Releases**: PRs merged from develop to main now automatically create draft releases with Docker images and tarballs
- **Enhanced Release Documentation**: Comprehensive release instructions now ensure consistent, safe release practices
- **Changelog System**: Flexible changelog generation supporting both automated and manual creation
- **Version Management**: Improved version tracking and validation throughout the release pipeline

### 📦 Build & Deployment

- **Multi-arch Docker Support**: Automated builds for linux/amd64 and linux/arm64
- **Release Tarballs**: Automatic creation of release archives with production dependencies
- **Draft-First Approach**: All releases are created as drafts, requiring manual review before publishing

### 🔧 Developer Experience

- **Simplified Workflows**: Streamlined release process with clear documentation
- **PR Templates**: Added templates to guide consistent pull request creation
- **Cleanup**: Removed obsolete files and configurations from the repository

### Update Instructions

**For existing installations:**
- Web Interface: Settings → System → Check for Updates
- Script: `./install.sh --update`
- Docker: `docker pull rcourtman/pulse:rc`

**Note**: This is a release candidate. Please test thoroughly before using in production.

---
*This release includes all changes from v3.31.0-rc1 plus the improvements listed above.*