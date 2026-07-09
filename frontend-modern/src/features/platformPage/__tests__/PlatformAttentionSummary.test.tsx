import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';
import { PlatformAttentionSummary } from '../PlatformAttentionSummary';

afterEach(cleanup);

describe('PlatformAttentionSummary', () => {
  it('renders an actionable compact posture region', () => {
    render(() => (
      <PlatformAttentionSummary
        title="Platform attention"
        headline="2 resources need review"
        description="Review the affected resources first."
        tone="warning"
        metrics={[
          { label: 'critical', value: 0 },
          { label: 'warning', value: 2 },
        ]}
        actions={<button type="button">Show attention</button>}
      />
    ));

    const region = screen.getByRole('region', { name: 'Platform attention' });
    expect(region).toHaveAttribute('data-platform-attention-summary', 'warning');
    expect(region).toHaveTextContent('2 resources need review');
    expect(screen.getByRole('button', { name: 'Show attention' })).toBeInTheDocument();
  });
});
