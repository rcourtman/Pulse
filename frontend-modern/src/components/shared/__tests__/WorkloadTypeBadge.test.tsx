import { afterEach, describe, expect, it } from 'vitest';
import { cleanup, render, screen } from '@solidjs/testing-library';
import { WorkloadTypeBadge } from '@/components/shared/WorkloadTypeBadge';
import { getWorkloadTypeBadge } from '@/components/shared/workloadTypeBadges';

afterEach(cleanup);

describe('WorkloadTypeBadge', () => {
  it('renders through the canonical workload type badge presentation', () => {
    render(() => <WorkloadTypeBadge type="system-container" />);

    const badge = screen.getByText('LXC');
    const presentation = getWorkloadTypeBadge('system-container');

    expect(badge).toHaveAttribute('title', presentation.title);
    for (const className of presentation.className.split(' ')) {
      expect(badge).toHaveClass(className);
    }
  });

  it('allows context-specific labels without changing the canonical tone', () => {
    render(() => <WorkloadTypeBadge type="agent" label="Host" title="Host backup" />);

    const badge = screen.getByText('Host');
    const presentation = getWorkloadTypeBadge('agent');

    expect(badge).toHaveAttribute('title', 'Host backup');
    for (const className of presentation.className.split(' ')) {
      expect(badge).toHaveClass(className);
    }
  });
});
