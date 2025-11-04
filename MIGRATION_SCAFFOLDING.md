# Migration Scaffolding Tracker

This document tracks temporary code added to handle migration paths between versions. All code listed here should be removed according to the specified criteria.

**Purpose**: These features exist solely to assist users in migrating from old patterns to new ones. They serve no functional purpose beyond migration assistance and represent technical debt that should be cleaned up once the migration period is complete.

---

## Active Migration Code

## Removed Migration Code

- Legacy SSH detection banner (removed in cleanup)

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
