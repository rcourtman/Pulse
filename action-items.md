# Remaining Action Items for Pulse

## Immediate Actions Needed

### 1. Respond to GitHub Issues
- [ ] Post migration guide link to issue #248
- [ ] Post installation workarounds to issues #251 and #252
- [ ] Close issues #249 and #250 as fixed in v4.0.1

### 2. Monitor PR Status
- [ ] Check status of Proxmox VE helper script PR
- [ ] Once merged, update README to remove warning

### 3. Consider v4.0.2 Release
Given the documentation improvements and user confusion, consider releasing v4.0.2 with:
- Migration guide included
- Updated README with clear warnings
- Helper script status documented

## Known Issues to Track

### v4 Issues
- Fresh installs failing due to helper script (PR submitted)
- Migration documentation was missing (now added)

### v3 Issues (Not our focus)
- #243: Security mode password issues
- #245: Can't add 3rd node (limited to 2 nodes)
- #246: v3.42.0 build broken

## Future Improvements
1. **Update install.sh** to better detect and handle v3 installations
2. **Add authentication** to v4 (currently no login system)
3. **Create automated tests** to prevent installation issues
4. **Update Docker Hub** description with migration warnings

## Communication Plan
1. Update release notes for v4.0.0 and v4.0.1 to link to migration guide
2. Consider pinning an issue about v3→v4 migration
3. Add migration warning to Docker Hub description