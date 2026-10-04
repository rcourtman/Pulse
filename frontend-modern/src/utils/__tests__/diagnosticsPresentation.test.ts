import { describe, expect, it } from 'vitest';
import {
  DIAGNOSTICS_EMPTY_PBS_MESSAGE,
  DIAGNOSTICS_EMPTY_STATE_COPY,
  DIAGNOSTICS_PANEL_COPY,
} from '@/utils/diagnosticsPresentation';

describe('diagnosticsPresentation', () => {
  it('exports canonical diagnostics panel framing copy', () => {
    expect(DIAGNOSTICS_PANEL_COPY).toMatchObject({
      title: 'System Diagnostics',
      description: 'Review connection health, configuration status, and troubleshooting tools.',
      summary: 'Collect runtime information and check configured Proxmox and PBS connections.',
      runActionLabel: 'Run Diagnostics',
      runShortLabel: 'Run',
      runningActionLabel: 'Running...',
      exportFullLabel: 'Full (private)',
      exportGithubLabel: 'GitHub (review first)',
      versionLabel: 'Version',
      uptimeLabel: 'Uptime',
      recommendedVersionLabel: 'Recommended version',
    });
    expect(DIAGNOSTICS_PANEL_COPY.runSafety).toContain('live API and guest-agent requests');
    expect(DIAGNOSTICS_PANEL_COPY.runSafety).toContain('backup, freeze/thaw');
    expect(DIAGNOSTICS_PANEL_COPY.runSafety).toContain('does not prove ongoing collection');
    expect(DIAGNOSTICS_PANEL_COPY.exportSafety).toContain('Nothing is uploaded');
    expect(DIAGNOSTICS_PANEL_COPY.exportSafety).toContain('Review even a sanitised file');
    expect(DIAGNOSTICS_PANEL_COPY.runSafety).not.toContain(';');
    expect(DIAGNOSTICS_PANEL_COPY.exportSafety).not.toContain(';');
  });

  it('exports canonical diagnostics empty-state copy', () => {
    expect(DIAGNOSTICS_EMPTY_STATE_COPY).toEqual({
      title: 'No diagnostics data available',
      description:
        'Collect diagnostics only when live checks are safe. Keep existing evidence if the host is unresponsive or a backup is running.',
      actionLabel: 'Run Diagnostics',
    });
    expect(DIAGNOSTICS_EMPTY_PBS_MESSAGE).toBe('No Proxmox Backup Server instances configured.');
  });
});
