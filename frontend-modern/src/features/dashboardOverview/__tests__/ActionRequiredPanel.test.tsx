import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';
import {
  getFindingCompactBadgePresentation,
  getFindingManualControlsPresentation,
  getFindingPrimaryActionPresentation,
  getFindingTitlePresentation,
} from '@/utils/aiFindingPresentation';

const actionRequiredPanelSource = readFileSync(
  resolve(__dirname, '..', 'ActionRequiredPanel.tsx'),
  'utf-8',
);

describe('ActionRequiredPanel', () => {
  it('uses runtime-qualified compact badges for Patrol runtime findings', () => {
    expect(
      getFindingCompactBadgePresentation({
        severity: 'warning',
        resourceId: 'ai-service',
        resourceName: 'Pulse Patrol Service',
        title: 'Pulse Patrol: Insufficient API credits',
      }),
    ).toEqual({
      label: 'Runtime issue',
      badgeClasses:
        'border-sky-200 bg-sky-50 text-sky-700 dark:border-sky-800 dark:bg-sky-900 dark:text-sky-300',
    });
  });

  it('keeps compact severity labels for infrastructure findings', () => {
    expect(
      getFindingCompactBadgePresentation({
        severity: 'warning',
        resourceId: 'vm-101',
        resourceName: 'db-01',
        title: 'Disk nearly full',
      }),
    ).toEqual({
      label: 'WARN',
      badgeClasses:
        'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-800 dark:bg-amber-900 dark:text-amber-300',
    });
  });

  it('routes dashboard finding rows through the shared Patrol title and action helpers', () => {
    expect(actionRequiredPanelSource).toContain(
      'const compactBadge = getFindingCompactBadgePresentation(finding);',
    );
    expect(actionRequiredPanelSource).toContain(
      'const title = getFindingTitlePresentation(finding).label;',
    );
    expect(actionRequiredPanelSource).toContain(
      'const primaryAction = getFindingPrimaryActionPresentation(finding);',
    );
    expect(actionRequiredPanelSource).toContain(
      'const manualControls = getFindingManualControlsPresentation(finding);',
    );
    expect(actionRequiredPanelSource).toContain('{compactBadge.label}');
    expect(actionRequiredPanelSource).toContain('title={title}');
    expect(actionRequiredPanelSource).toContain('{title}');
  });

  it('uses the Patrol provider settings action instead of rejected generic controls for Patrol runtime findings', () => {
    expect(
      getFindingTitlePresentation({
        resourceId: 'ai-service',
        resourceName: 'Pulse Patrol Service',
        title: 'Pulse Patrol: Insufficient API credits',
      }),
    ).toEqual({
      label: 'Insufficient API credits',
    });
    expect(
      getFindingPrimaryActionPresentation({
        resourceId: 'ai-service',
        resourceName: 'Pulse Patrol Service',
        title: 'Pulse Patrol: Insufficient API credits',
      }),
    ).toEqual({
      label: 'Open Patrol provider settings',
      href: '/settings/system-ai',
    });
    expect(
      getFindingManualControlsPresentation({
        resourceId: 'ai-service',
        resourceName: 'Pulse Patrol Service',
        title: 'Pulse Patrol: Insufficient API credits',
      }),
    ).toEqual({
      acknowledge: false,
      snooze: false,
      dismiss: false,
    });
    expect(actionRequiredPanelSource).toContain('when={primaryAction}');
    expect(actionRequiredPanelSource).toContain('{action().label}');
    expect(actionRequiredPanelSource).toContain('manualControls.snooze');
    expect(actionRequiredPanelSource).toContain('manualControls.dismiss');
  });
});
