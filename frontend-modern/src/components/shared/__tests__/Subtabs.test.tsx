import { fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
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
    expect(tablist).toHaveClass('gap-3', 'sm:gap-6');
    expect(tablist).not.toHaveClass('flex-wrap');

    const historyTab = within(tablist).getByRole('tab', { name: 'History' });
    expect(historyTab).toHaveAttribute('aria-selected', 'true');
    expect(historyTab).toHaveClass('shrink-0');
    expect(historyTab).toHaveClass('whitespace-nowrap');
    expect(historyTab).toHaveClass('min-h-9', 'text-xs', 'sm:min-h-10', 'sm:text-sm');
  });

  it('scrolls a newly selected tab into view', async () => {
    const previousScrollIntoView = Element.prototype.scrollIntoView;
    const scrollIntoView = vi.fn();
    Object.defineProperty(Element.prototype, 'scrollIntoView', {
      configurable: true,
      value: scrollIntoView,
    });

    try {
      const [value, setValue] = createSignal('overview');
      render(() => (
        <Subtabs
          value={value()}
          onChange={setValue}
          ariaLabel="Threshold platform"
          tabs={[
            { value: 'overview', label: 'Overview' },
            { value: 'kubernetes', label: 'Kubernetes' },
            { value: 'systems', label: 'Machines' },
          ]}
        />
      ));

      scrollIntoView.mockClear();
      fireEvent.click(screen.getByRole('tab', { name: 'Machines' }));

      await waitFor(() => {
        expect(screen.getByRole('tab', { name: 'Machines' })).toHaveAttribute(
          'aria-selected',
          'true',
        );
        expect(scrollIntoView).toHaveBeenCalledWith({ block: 'nearest', inline: 'nearest' });
      });
    } finally {
      if (previousScrollIntoView) {
        Object.defineProperty(Element.prototype, 'scrollIntoView', {
          configurable: true,
          value: previousScrollIntoView,
        });
      } else {
        Reflect.deleteProperty(Element.prototype, 'scrollIntoView');
      }
    }
  });
});
