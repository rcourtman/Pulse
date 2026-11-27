import { describe, expect, it } from 'vitest';
import {
  getContainerUpdateUncheckableLabel,
  getUpdateUnavailableExplanation,
  hasContainerUpdateError,
  isContainerUpdateUncheckable,
  isHostCommandsDisabledUpdateRefusal,
} from '@/components/shared/containerUpdateBadgeModel';

const disconnected = {
  reason: 'Docker / Podman command agent is not connected.',
  reasonCode: 'command_agent_disconnected',
  hostName: 'Tower',
};

describe('locked update explanation', () => {
  it('keeps the tooltip concise and invites click-to-enable for admins when commands are off', () => {
    const text = getUpdateUnavailableExplanation({
      ...disconnected,
      hostCommandsEnabled: false,
      canEnableCommands: true,
    });
    expect(text).toBe('Update available. Click to enable remote commands on Tower.');
    expect(text).not.toContain('\n');
    expect(text).not.toContain('Settings');
  });

  it('states read-only status concisely when the viewer cannot enable commands', () => {
    const text = getUpdateUnavailableExplanation({
      reason: 'Commands are not enabled on the Pulse agent on Tower.',
      reasonCode: 'host_commands_disabled',
      hostName: 'Tower',
      canEnableCommands: false,
    });
    expect(text).toBe('Update available. Remote commands are off on Tower.');
    expect(text).not.toContain('Click to enable');
  });

  it('identifies commands-off refusals from either the explicit code or legacy disconnected + flag', () => {
    expect(isHostCommandsDisabledUpdateRefusal({ reasonCode: 'host_commands_disabled' })).toBe(
      true,
    );
    expect(
      isHostCommandsDisabledUpdateRefusal({
        reasonCode: 'command_agent_disconnected',
        hostCommandsEnabled: false,
      }),
    ).toBe(true);
    expect(
      isHostCommandsDisabledUpdateRefusal({
        reasonCode: 'command_agent_disconnected',
        hostCommandsEnabled: true,
      }),
    ).toBe(false);
  });

  it('tells a lost command channel apart from missing access', () => {
    const text = getUpdateUnavailableExplanation({ ...disconnected, hostCommandsEnabled: true });
    expect(text).toBe('Update available. Command channel to Tower is offline.');
    expect(text).not.toContain('Remote commands');
  });

  it("falls back to a single concise line using the server's reason when command access is unknown", () => {
    expect(getUpdateUnavailableExplanation(disconnected)).toBe(
      'Update available. Docker / Podman command agent is not connected.',
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
