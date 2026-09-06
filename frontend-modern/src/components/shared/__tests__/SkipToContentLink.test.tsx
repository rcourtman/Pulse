import { fireEvent, render, screen } from '@solidjs/testing-library';
import { describe, expect, it } from 'vitest';
import appSource from '@/App.tsx?raw';
import { SkipToContentLink } from '@/components/shared/SkipToContentLink';

describe('SkipToContentLink', () => {
  it('moves focus to the main landmark and reveals itself only while focused', () => {
    render(() => (
      <>
        <SkipToContentLink />
        <main id="main" tabindex="-1">
          Content
        </main>
      </>
    ));

    const skipLink = screen.getByRole('link', { name: 'Skip to main content' });
    expect(skipLink).toHaveAttribute('href', '#main');
    expect(skipLink).toHaveClass('sr-only');

    fireEvent.focus(skipLink);
    expect(skipLink).not.toHaveClass('sr-only');
    fireEvent.blur(skipLink);
    expect(skipLink).toHaveClass('sr-only');

    fireEvent.click(skipLink);
    expect(screen.getByRole('main')).toHaveFocus();
  });

  it('is rendered by the app shell ahead of the global banners', () => {
    // Tab order follows DOM order. The update, security and demo banners
    // render before AppLayout, so the link has to sit above that banner block
    // or the first Tab lands on a banner control instead.
    const skipLinkIndex = appSource.indexOf('<SkipToContentLink />');
    const bannerBlockIndex = appSource.indexOf('<SecurityWarning />');
    const layoutIndex = appSource.indexOf('<AppLayout');
    expect(skipLinkIndex).toBeGreaterThan(-1);
    expect(bannerBlockIndex).toBeGreaterThan(skipLinkIndex);
    expect(layoutIndex).toBeGreaterThan(bannerBlockIndex);
  });
});
