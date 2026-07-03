import { describe, expect, it } from 'vitest';
import { PVE_MANUAL_PERMISSION_COMMAND } from '../nodeModalModel';

describe('nodeModalModel', () => {
  it('prefers PVE 9 guest-agent privileges before legacy VM.Monitor', () => {
    expect(PVE_MANUAL_PERMISSION_COMMAND).toContain('VM.GuestAgent.Audit');
    expect(PVE_MANUAL_PERMISSION_COMMAND).toContain('VM.GuestAgent.FileRead');
    expect(PVE_MANUAL_PERMISSION_COMMAND).toContain('VM.Monitor');

    const guestBranch = PVE_MANUAL_PERMISSION_COMMAND.indexOf(
      'if [ "$HAS_GUEST_AGENT_AUDIT" = true ]; then',
    );
    const monitorBranch = PVE_MANUAL_PERMISSION_COMMAND.indexOf(
      'pveum role add PulseTmpVMMonitor -privs VM.Monitor',
    );

    expect(guestBranch).toBeGreaterThanOrEqual(0);
    expect(monitorBranch).toBeGreaterThanOrEqual(0);
    expect(guestBranch).toBeLessThan(monitorBranch);
  });
});
