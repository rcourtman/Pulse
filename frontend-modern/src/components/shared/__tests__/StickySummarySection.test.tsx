import { render, screen } from '@solidjs/testing-library';
import { describe, expect, it } from 'vitest';
import { StickySummarySection } from '@/components/shared/StickySummarySection';

describe('StickySummarySection', () => {
  it('keeps summaries sticky by default on desktop-only surfaces', () => {
    render(() => (
      <StickySummarySection>
        <div>Summary content</div>
      </StickySummarySection>
    ));

    const section = screen.getByText('Summary content').closest('[data-sticky-summary="true"]');
    expect(section).toHaveAttribute('data-sticky-summary-desktop-only', 'true');
    expect(section).toHaveAttribute('data-sticky-summary-sticky-desktop-only', 'false');
    expect(section?.className).toContain('hidden');
    expect(section?.className).toContain('lg:block');
    expect(section?.className).toContain('sticky');
  });

  it('can render below desktop while only becoming sticky at desktop width', () => {
    render(() => (
      <StickySummarySection desktopOnly={false} stickyDesktopOnly>
        <div>Storage charts</div>
      </StickySummarySection>
    ));

    const section = screen.getByText('Storage charts').closest('[data-sticky-summary="true"]');
    expect(section).toHaveAttribute('data-sticky-summary-desktop-only', 'false');
    expect(section).toHaveAttribute('data-sticky-summary-sticky-desktop-only', 'true');
    expect(section?.className).not.toContain('hidden');
    expect(section?.className).toContain('static');
    expect(section?.className).toContain('lg:sticky');
  });
});
