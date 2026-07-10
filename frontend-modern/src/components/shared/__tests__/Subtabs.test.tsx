import { render, screen, within } from '@solidjs/testing-library';
import { describe, expect, it, vi } from 'vitest';

import { Subtabs } from '@/components/shared/Subtabs';

describe('Subtabs', () => {
  it('keeps long tab sets on one scrollable row', () => {
    render(() => (
      <Subtabs
        value="history"
        onChange={vi.fn()}
        ariaLabel="Resource detail sections"
        tabs={[
          { value: 'overview', label: 'Overview' },
          { value: 'performance', label: 'Performance' },
          { value: 'history', label: 'History' },
        ]}
      />
    ));

    const tablist = screen.getByRole('tablist', { name: 'Resource detail sections' });
    expect(tablist).toHaveClass('overflow-x-auto');
    expect(tablist).toHaveClass('scrollbar-hide');
    expect(tablist).not.toHaveClass('flex-wrap');

    const historyTab = within(tablist).getByRole('tab', { name: 'History' });
    expect(historyTab).toHaveAttribute('aria-selected', 'true');
    expect(historyTab).toHaveClass('shrink-0');
    expect(historyTab).toHaveClass('whitespace-nowrap');
  });
});
