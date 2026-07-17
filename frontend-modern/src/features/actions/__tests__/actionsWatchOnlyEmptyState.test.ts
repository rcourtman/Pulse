import { describe, expect, it } from 'vitest';

import { getActionsWatchOnlyEmptyState } from '../actionPresentation';

const base = {
  patrolWatchOnly: true,
  patrolModesUnlocked: false,
  commercialSurfacesHidden: false,
  upgradePromptsHidden: false,
  upgradeDestination: { href: '/settings/billing', external: false },
};

describe('getActionsWatchOnlyEmptyState', () => {
  it('returns nothing when Patrol is not in Watch only', () => {
    expect(getActionsWatchOnlyEmptyState({ ...base, patrolWatchOnly: false })).toBeUndefined();
    expect(
      getActionsWatchOnlyEmptyState({
        ...base,
        patrolWatchOnly: false,
        patrolModesUnlocked: true,
      }),
    ).toBeUndefined();
  });

  it('points unlocked installs at the Ask first switch on the Patrol page', () => {
    expect(getActionsWatchOnlyEmptyState({ ...base, patrolModesUnlocked: true })).toEqual({
      kind: 'switch',
      body: 'Patrol runs in Watch only mode, so it reports issues without preparing fixes. Switch Patrol to Ask first and proposed fixes will wait here for your approval.',
      actionLabel: 'Open Patrol',
    });
  });

  it('explains the Pro capability for locked installs with an upgrade action', () => {
    expect(getActionsWatchOnlyEmptyState(base)).toEqual({
      kind: 'upgrade',
      body: 'Patrol runs in Watch only mode. Pulse Pro lets Patrol investigate issues and prepare fixes that wait here for your approval.',
      actionLabel: 'Learn about Pulse Pro',
      destination: base.upgradeDestination,
    });
  });

  it('keeps the explanation but drops the upgrade action when upgrade prompts are hidden', () => {
    const presentation = getActionsWatchOnlyEmptyState({ ...base, upgradePromptsHidden: true });
    expect(presentation).toMatchObject({ kind: 'upgrade' });
    expect(presentation).not.toHaveProperty('actionLabel');
    expect(presentation).not.toHaveProperty('destination');
  });

  it('stays silent for locked installs when commercial surfaces are hidden', () => {
    expect(
      getActionsWatchOnlyEmptyState({ ...base, commercialSurfacesHidden: true }),
    ).toBeUndefined();
  });
});
