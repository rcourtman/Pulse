import { describe, expect, it } from 'vitest';

import { getPatrolInvestigationVerdict } from '../patrolInvestigationContextModel';

const SUMMARY = `### Investigation Summary
nextcloud (app-container-4c434064ff5fc355) is running but unhealthy.

### Root Cause
Causal resource: nextcloud-db (app-container-e4d4620ea8858185). The database is stopped.

### Recommendation
A start action on nextcloud-db has been proposed.

### Conclusion
NEEDS_ATTENTION: Proposed a start on nextcloud-db (app-container-e4d4620ea8858185), the stopped
database behind nextcloud's failing health check. It waits for approval.`;

describe('getPatrolInvestigationVerdict', () => {
  it('returns the conclusion line without its outcome marker or registry ids', () => {
    expect(getPatrolInvestigationVerdict(SUMMARY)).toEqual({
      text: "Proposed a start on nextcloud-db, the stopped database behind nextcloud's failing health check. It waits for approval.",
    });
  });

  it('reads each outcome marker, bold or plain', () => {
    expect(
      getPatrolInvestigationVerdict('## Conclusion\nRESOLVED: the disk has space again'),
    ).toEqual({ text: 'the disk has space again' });
    expect(
      getPatrolInvestigationVerdict('## Conclusion\n**CANNOT_FIX**: only a person can replace it'),
    ).toEqual({ text: 'only a person can replace it' });
  });

  it('drops identifiers listed beside a name', () => {
    expect(
      getPatrolInvestigationVerdict(
        '### Conclusion\nNEEDS_ATTENTION: An admin must approve the pending "start" action on nextcloud-db (app-container-f678dd3b3f36f8ee, action_id act_a6336838f685c101dde39e1852e2e7be) to restore the database.',
      ),
    ).toEqual({
      text: 'An admin must approve the pending "start" action on nextcloud-db to restore the database.',
    });
  });

  it('stops at the next heading', () => {
    expect(
      getPatrolInvestigationVerdict('### Conclusion\nNEEDS_ATTENTION: start it\n### Notes\nlater'),
    ).toEqual({ text: 'start it' });
  });

  it('returns null without a conclusion section', () => {
    // A finding nobody investigated carries its first-look description here.
    expect(getPatrolInvestigationVerdict('nextcloud is unhealthy')).toBeNull();
    expect(getPatrolInvestigationVerdict('### Conclusion\n\n')).toBeNull();
    expect(getPatrolInvestigationVerdict(undefined)).toBeNull();
    expect(getPatrolInvestigationVerdict(null)).toBeNull();
  });
});
