import { describe, expect, it, afterEach } from 'vitest';
import { render, screen, cleanup } from '@solidjs/testing-library';
import { DeployStatusBadge } from '../deploy/DeployStatusBadge';
import type { DeployTargetStatus } from '@/types/agentDeploy';

afterEach(() => cleanup());

describe('DeployStatusBadge', () => {
  const cases: [DeployTargetStatus, string][] = [
    ['pending', 'Pending'],
    ['preflighting', 'Checking'],
    ['ready', 'Ready'],
    ['installing', 'Installing'],
    ['enrolling', 'Enrolling'],
    ['verifying', 'Verifying'],
    ['succeeded', 'Deployed'],
    ['failed_retryable', 'Failed'],
    ['failed_permanent', 'Failed'],
    ['skipped_already_agent', 'Already monitored'],
    ['skipped_license', 'License limit'],
    ['canceled', 'Canceled'],
  ];

  it.each(cases)('renders "%s" status as "%s"', (status, label) => {
    render(() => <DeployStatusBadge status={status} />);
    expect(screen.getByText(label)).toBeInTheDocument();
  });

  it('applies animate-pulse for in-progress statuses', () => {
    const inProgress: DeployTargetStatus[] = [
      'preflighting',
      'installing',
      'enrolling',
      'verifying',
    ];
    for (const status of inProgress) {
      const { unmount } = render(() => <DeployStatusBadge status={status} />);
      const badge = screen.getByText(
        status === 'preflighting'
          ? 'Checking'
          : status === 'installing'
            ? 'Installing'
            : status === 'enrolling'
              ? 'Enrolling'
              : 'Verifying',
      );
      expect(badge.className).toContain('animate-pulse');
      unmount();
    }
  });

  it('does not apply animate-pulse for terminal statuses', () => {
    const terminal: DeployTargetStatus[] = ['succeeded', 'failed_retryable', 'canceled'];
    for (const status of terminal) {
      const { unmount } = render(() => <DeployStatusBadge status={status} />);
      const badge = screen.getByText(
        status === 'succeeded' ? 'Deployed' : status === 'failed_retryable' ? 'Failed' : 'Canceled',
      );
      expect(badge.className).not.toContain('animate-pulse');
      unmount();
    }
  });

  it('uses theme tokens (not gray-*) for pending and canceled', () => {
    for (const status of ['pending', 'canceled'] as DeployTargetStatus[]) {
      const { unmount } = render(() => <DeployStatusBadge status={status} />);
      const badge = screen.getByText(status === 'pending' ? 'Pending' : 'Canceled');
      expect(badge.className).toContain('bg-surface-alt');
      expect(badge.className).toContain('text-muted');
      expect(badge.className).not.toContain('gray');
      unmount();
    }
  });
});
