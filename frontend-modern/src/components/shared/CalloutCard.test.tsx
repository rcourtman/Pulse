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

  it('supports compact settings callout scale', () => {
    render(() => (
      <CalloutCard
        title="Compact notice"
        description="Fits in dense settings panels"
        tone="danger"
        scale="compact"
        padding="sm"
      />
    ));

    expect(screen.getByText('Compact notice').className).toContain('text-sm');
    expect(screen.getByText('Fits in dense settings panels').className).toContain('text-xs');
  });
});
