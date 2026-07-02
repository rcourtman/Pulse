# Current Branch GA Owner Approval

- Decision date: 2026-07-02
- Release target: Pulse v6.0.0 GA
- Release branch: `pulse/v6-release`
- Scope decision: include accumulated post-RC7 changes in the GA release
- Additional RC or soak required: no
- Additional current-branch validation required before GA: no
- GA date for publication packet: 2026-07-02
- v5 end-of-support date for publication packet: 2026-09-30

## Owner Direction

The release owner explicitly approved shipping the current v6 release branch as
the GA scope even though the post-RC7 changes have not received another RC or
soak cycle.

This record is risk acceptance, not validation evidence. The release record
must not describe the post-RC7 changes as RC-tested. The public release packet
must describe v6.0.0 as the current branch GA after seven release candidates,
with the exact GA and v5 end-of-support dates updated for the actual July 2,
2026 launch decision.

## Release-Control Impact

For v6.0.0 only, `rc-to-ga-promotion-readiness` may be treated as cleared by
the combination of:

1. prior governed release-pipeline rehearsal evidence for v6.0.0,
2. explicit rollback target and reinstall command,
3. the written v5 maintenance-only policy,
4. the current-branch GA policy record, and
5. this owner approval accepting the remaining current-branch validation risk.

This bounded exception must not become the default stable-promotion rule for
future releases.
