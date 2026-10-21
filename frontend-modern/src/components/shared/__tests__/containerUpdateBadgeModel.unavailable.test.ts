import { describe, expect, it } from 'vitest';
import {
  getContainerUpdateUncheckableLabel,
  getUpdateUnavailableExplanation,
  hasContainerUpdateError,
  isContainerUpdateUncheckable,
} from '@/components/shared/containerUpdateBadgeModel';

const disconnected = {
  reason: 'Docker / Podman command agent is not connected.',
  reasonCode: 'command_agent_disconnected',
  hostName: 'Tower',
};

describe('locked update explanation', () => {
  it('names a missing command access and where to turn it on', () => {
    const text = getUpdateUnavailableExplanation({ ...disconnected, hostCommandsEnabled: false });
    expect(text).toContain('the Pulse agent on Tower was installed without command access');
    expect(text).toContain('Settings → Infrastructure');
    expect(text).toContain(
      '"Allow Pulse to run commands on this host" selected and run it on Tower',
    );
    expect(text).not.toContain('advanced');
    expect(text).not.toContain('not connected');
  });

  it('tells a lost command channel apart from missing access', () => {
    const text = getUpdateUnavailableExplanation({ ...disconnected, hostCommandsEnabled: true });
    expect(text).toContain('the command channel to the Pulse agent on Tower is not connected');
    expect(text).not.toContain('Allow Pulse to run commands');
  });

  it("falls back to the server's reason when command access is unknown", () => {
    expect(getUpdateUnavailableExplanation(disconnected)).toBe(
      'Update available. Pulse cannot apply it: Docker / Podman command agent is not connected.\n' +
        'Apply this update using your container manager on the host.',
    );
    expect(
      getUpdateUnavailableExplanation({
        reason: 'Docker / Podman inventory is not fresh enough to run lifecycle actions.',
        reasonCode: 'stale_inventory',
        hostCommandsEnabled: false,
      }),
    ).toContain('inventory is not fresh enough');
  });
});

describe('uncheckable images', () => {
  it('reads a local image as its own state, not a failed check', () => {
    const local = { updateAvailable: false, lastChecked: 0, error: 'local image' };
    expect(isContainerUpdateUncheckable(local)).toBe(true);
    expect(hasContainerUpdateError(local)).toBe(false);
    expect(getContainerUpdateUncheckableLabel(local)).toBe('Local');
    expect(getContainerUpdateUncheckableLabel({ error: 'digest-pinned image' })).toBe('Pinned');
    expect(
      hasContainerUpdateError({
        updateAvailable: false,
        lastChecked: 0,
        error: 'authentication required',
      }),
    ).toBe(true);
  });
});
