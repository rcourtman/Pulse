import { describe, expect, it } from 'vitest';
import {
  DIAGNOSTICS_EMPTY_PBS_MESSAGE,
  DIAGNOSTICS_EMPTY_STATE_COPY,
  DIAGNOSTICS_PANEL_COPY,
} from '@/utils/diagnosticsPresentation';

describe('diagnosticsPresentation', () => {
  it('exports canonical diagnostics panel framing copy', () => {
    expect(DIAGNOSTICS_PANEL_COPY).toEqual({
      title: 'System Diagnostics',
      description: 'Review connection health, configuration status, and troubleshooting tools.',
      summary: 'Test all connections and inspect runtime configuration.',
      runActionLabel: 'Run Diagnostics',
      runShortLabel: 'Run',
      runningActionLabel: 'Running...',
      exportFullLabel: 'Full',
      exportGithubLabel: 'GitHub',
      versionLabel: 'Version',
      uptimeLabel: 'Uptime',
      recommendedVersionLabel: 'Recommended version',
    });
  });

  it('exports canonical diagnostics empty-state copy', () => {
    expect(DIAGNOSTICS_EMPTY_STATE_COPY).toEqual({
      title: 'No diagnostics data available',
      description: 'Run diagnostics to test connections and inspect system status.',
      actionLabel: 'Run Diagnostics',
    });
    expect(DIAGNOSTICS_EMPTY_PBS_MESSAGE).toBe(
      'No Proxmox Backup Server instances configured.',
    );
  });
});
