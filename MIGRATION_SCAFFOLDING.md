# Migration Scaffolding Tracker

This document tracks temporary code added to handle migration paths between versions. All code listed here should be removed according to the specified criteria.

**Purpose**: These features exist solely to assist users in migrating from old patterns to new ones. They serve no functional purpose beyond migration assistance and represent technical debt that should be cleaned up once the migration period is complete.

---

## Active Migration Code

### Legacy SSH Detection Banner (Added: v4.23.0)

**Why it exists**: Users who set up temperature monitoring before v4.23.0 used SSH keys directly in the container. The new secure architecture uses `pulse-sensor-proxy` on the host with Unix socket communication. Users with the old setup need to remove and re-add their nodes to upgrade.

**Files involved**:
- Backend: `/opt/pulse/internal/api/router.go` - `detectLegacySSH()` function
- Backend: `/opt/pulse/internal/api/types.go` - `HealthResponse.LegacySSHDetected` fields
- Frontend: `/opt/pulse/frontend-modern/src/components/LegacySSHBanner.tsx`
- Frontend: `/opt/pulse/frontend-modern/src/App.tsx` - Banner integration

**Removal criteria** (ANY of these):
- Version reaches v5.0 or later
- Telemetry shows <1% detection rate for 30+ consecutive days
- 6 months after v4.23.0 release (whichever comes first)

**How to remove**:
1. Delete `LegacySSHBanner.tsx` component
2. Remove import and usage from `App.tsx`
3. Delete `detectLegacySSH()` function from `router.go`
4. Remove `LegacySSHDetected`, `RecommendProxyUpgrade`, `ProxyInstallScriptAvailable` from `types.go` HealthResponse
5. Remove banner logic from `handleHealth()` in `router.go`
6. Delete this entry from this document

**Manual override**: Set `PULSE_LEGACY_DETECTION=false` environment variable to disable detection without code changes.

**Telemetry notes**: Currently no telemetry implemented. Consider adding metrics to track:
- How often the banner is shown
- How many users still have legacy SSH setup
- When detection rate drops below threshold

---

## Removed Migration Code

(None yet - this document was created v4.23.0)

---

## Guidelines for Adding New Migration Code

When adding new migration scaffolding:

1. **Mark it clearly** with `⚠️ MIGRATION SCAFFOLDING - TEMPORARY CODE` comments
2. **Add it to this document** with:
   - Why it exists
   - Files involved
   - Removal criteria
   - Removal instructions
3. **Set explicit removal criteria**:
   - Version number target
   - Time-based target (e.g., "6 months after release")
   - Data-driven target (e.g., "when <1% users affected")
4. **Add a kill switch**: Environment variable or feature flag to disable without redeployment
5. **Consider telemetry**: If removal depends on data, instrument the code to collect that data
6. **Create a removal task**: Add to issue tracker with assigned owner and date

**Remember**: Migration code is technical debt. Make it easy to find and remove later.
