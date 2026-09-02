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

  it('moves focus across enabled tabs with standard tab-list keys without changing selection', () => {
    const onChange = vi.fn();
    render(() => (
      <Subtabs
        value="overview"
        onChange={onChange}
        ariaLabel="Resource detail sections"
        tabs={[
          { value: 'overview', label: 'Overview' },
          { value: 'performance', label: 'Performance', disabled: true },
          { value: 'history', label: 'History' },
          { value: 'manage', label: 'Manage' },
        ]}
      />
    ));

    const overview = screen.getByRole('tab', { name: 'Overview' });
    const history = screen.getByRole('tab', { name: 'History' });
    const manage = screen.getByRole('tab', { name: 'Manage' });

    overview.focus();
    fireEvent.keyDown(overview, { key: 'ArrowRight' });
    expect(history).toHaveFocus();

    fireEvent.keyDown(history, { key: 'End' });
    expect(manage).toHaveFocus();

    fireEvent.keyDown(manage, { key: 'ArrowRight' });
    expect(overview).toHaveFocus();

    fireEvent.keyDown(overview, { key: 'ArrowLeft' });
    expect(manage).toHaveFocus();

    fireEvent.keyDown(manage, { key: 'Home' });
    expect(overview).toHaveFocus();
    expect(overview).toHaveAttribute('aria-selected', 'true');
    expect(onChange).not.toHaveBeenCalled();
  });

  it('keeps keyboard-focused tabs available for manual activation', () => {
    const onChange = vi.fn();
    render(() => (
      <Subtabs
        value="overview"
        onChange={onChange}
        ariaLabel="Resource detail sections"
        tabs={[
          { value: 'overview', label: 'Overview' },
          { value: 'history', label: 'History' },
        ]}
      />
    ));

    const overview = screen.getByRole('tab', { name: 'Overview' });
    const history = screen.getByRole('tab', { name: 'History' });
    overview.focus();
    fireEvent.keyDown(overview, { key: 'ArrowRight' });
    fireEvent.click(history);

    expect(history).toHaveFocus();
    expect(onChange).toHaveBeenCalledOnce();
    expect(onChange).toHaveBeenCalledWith('history');
  });

  it('shows phone scroll affordances when the tab rail is clipped', async () => {
    render(() => (
      <Subtabs
        value="overview"
        onChange={vi.fn()}
        ariaLabel="Resource detail sections"
        tabs={[
          { value: 'overview', label: 'Overview' },
          { value: 'history', label: 'History' },
          { value: 'manage', label: 'Manage' },
          { value: 'deployments', label: 'Deployments' },
        ]}
      />
    ));

    const tablist = screen.getByRole('tablist', { name: 'Resource detail sections' });
    const scrollBy = vi.fn();
    Object.defineProperties(tablist, {
      clientWidth: { configurable: true, value: 180 },
      scrollWidth: { configurable: true, value: 420 },
      scrollLeft: { configurable: true, writable: true, value: 0 },
      scrollBy: { configurable: true, value: scrollBy },
    });
    window.dispatchEvent(new Event('resize'));

    const scrollRight = await screen.findByRole('button', {
      name: 'Resource detail sections: scroll right',
    });
    expect(scrollRight).toHaveClass('sm:hidden');
    expect(screen.queryByRole('button', { name: /scroll left/i })).not.toBeInTheDocument();

    await fireEvent.click(scrollRight);
    expect(scrollBy).toHaveBeenCalledWith({ left: 126, behavior: 'smooth' });

    tablist.scrollLeft = 120;
    tablist.dispatchEvent(new Event('scroll'));
    expect(
      await screen.findByRole('button', { name: 'Resource detail sections: scroll left' }),
    ).toBeInTheDocument();
  });
});
