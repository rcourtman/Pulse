import { describe, expect, it, afterEach } from 'vitest';
import { render, screen, cleanup } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
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

  it('does not apply animate-pulse for non-progress statuses', () => {
    const nonProgress: [DeployTargetStatus, string][] = [
      ['pending', 'Pending'],
      ['ready', 'Ready'],
      ['succeeded', 'Deployed'],
      ['failed_retryable', 'Failed'],
      ['failed_permanent', 'Failed'],
      ['skipped_already_agent', 'Already monitored'],
      ['skipped_license', 'License limit'],
      ['canceled', 'Canceled'],
    ];
    for (const [status, label] of nonProgress) {
      const { unmount } = render(() => <DeployStatusBadge status={status} />);
      const badge = screen.getByText(label);
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

  it('applies emerald classes to success statuses (ready, succeeded)', () => {
    for (const status of ['ready', 'succeeded'] as DeployTargetStatus[]) {
      const { unmount } = render(() => <DeployStatusBadge status={status} />);
      const badge = screen.getByText(status === 'ready' ? 'Ready' : 'Deployed');
      expect(badge.className).toContain('bg-emerald-100');
      expect(badge.className).toContain('text-emerald-700');
      unmount();
    }
  });

  it('applies red classes to failure statuses', () => {
    for (const status of ['failed_retryable', 'failed_permanent'] as DeployTargetStatus[]) {
      const { unmount } = render(() => <DeployStatusBadge status={status} />);
      const badge = screen.getByText('Failed');
      expect(badge.className).toContain('bg-red-100');
      expect(badge.className).toContain('text-red-700');
      unmount();
    }
  });

  it('applies amber classes to skipped statuses', () => {
    for (const status of ['skipped_already_agent', 'skipped_license'] as DeployTargetStatus[]) {
      const { unmount } = render(() => <DeployStatusBadge status={status} />);
      const label = status === 'skipped_already_agent' ? 'Already monitored' : 'License limit';
      const badge = screen.getByText(label);
      expect(badge.className).toContain('bg-amber-100');
      expect(badge.className).toContain('text-amber-700');
      unmount();
    }
  });

  it('applies blue classes to in-progress statuses', () => {
    const labels: Record<string, string> = {
      preflighting: 'Checking',
      installing: 'Installing',
      enrolling: 'Enrolling',
      verifying: 'Verifying',
    };
    for (const status of Object.keys(labels) as DeployTargetStatus[]) {
      const { unmount } = render(() => <DeployStatusBadge status={status} />);
      const badge = screen.getByText(labels[status]);
      expect(badge.className).toContain('bg-blue-100');
      expect(badge.className).toContain('text-blue-700');
      unmount();
    }
  });

  it('renders as an inline-flex rounded-full span with whitespace-nowrap', () => {
    render(() => <DeployStatusBadge status="pending" />);
    const badge = screen.getByText('Pending');
    expect(badge.tagName).toBe('SPAN');
    expect(badge.className).toContain('inline-flex');
    expect(badge.className).toContain('rounded-full');
    expect(badge.className).toContain('whitespace-nowrap');
  });

  it('falls back to pending config for an unknown status value', () => {
    render(() => <DeployStatusBadge status={'bogus' as DeployTargetStatus} />);
    const badge = screen.getByText('Pending');
    expect(badge.className).toContain('bg-surface-alt');
    expect(badge.className).toContain('text-muted');
  });

  it('updates label and classes reactively when status changes', () => {
    const [status, setStatus] = createSignal<DeployTargetStatus>('pending');
    render(() => <DeployStatusBadge status={status()} />);

    const pendingBadge = screen.getByText('Pending');
    expect(pendingBadge.className).toContain('bg-surface-alt');

    setStatus('succeeded');
    const deployedBadge = screen.getByText('Deployed');
    expect(deployedBadge.className).toContain('bg-emerald-100');
    expect(deployedBadge.className).not.toContain('bg-surface-alt');
  });
});
