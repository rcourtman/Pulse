import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';
import { CalloutCard } from './CalloutCard';

describe('CalloutCard', () => {
  afterEach(() => {
    cleanup();
  });

  it('renders info callout content through the shared primitive', () => {
    render(() => (
      <CalloutCard
        icon={<span data-testid="info-icon">i</span>}
        title="Advanced Insights"
        description="Shared informational callout"
      />
    ));

    expect(screen.getByText('Advanced Insights')).toBeInTheDocument();
    expect(screen.getByText('Shared informational callout')).toBeInTheDocument();
    expect(screen.getByTestId('info-icon')).toBeInTheDocument();
  });

  it('supports warning tone styling', () => {
    render(() => (
      <CalloutCard title="Warning" description="Act carefully" tone="warning" />
    ));

    const description = screen.getByText('Act carefully');
    expect(description.className).toContain('text-amber-800');
  });
});
