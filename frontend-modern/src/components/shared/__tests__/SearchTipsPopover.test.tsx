import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { SearchTipsPopover } from '@/components/shared/SearchTipsPopover';

describe('SearchTipsPopover', () => {
  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

  it('toggles the popover on click by default', async () => {
    render(() => (
      <SearchTipsPopover tips={[{ code: 'name:web', description: 'Filter by name' }]} />
    ));

    const trigger = screen.getByRole('button', { name: 'Search tips' });
    expect(screen.queryByRole('dialog', { name: 'Search tips' })).toBeNull();

    fireEvent.click(trigger);
    const dialog = await screen.findByRole('dialog', { name: 'Search tips' });
    expect(dialog).toBeInTheDocument();
    expect(trigger).toHaveAttribute('aria-haspopup', 'dialog');
    expect(screen.getByText('name:web')).toHaveClass('whitespace-nowrap');

    fireEvent.click(trigger);
    expect(screen.queryByRole('dialog', { name: 'Search tips' })).toBeNull();
  });

  it('moves focus into an explicitly opened dialog and restores it on close', async () => {
    render(() => (
      <SearchTipsPopover openOnHover tips={[{ code: 'name:web', description: 'Filter by name' }]} />
    ));

    const trigger = screen.getByRole('button', { name: 'Search tips' });
    trigger.focus();
    expect(screen.queryByRole('dialog', { name: 'Search tips' })).toBeNull();
    fireEvent.click(trigger);

    const close = await screen.findByRole('button', { name: 'Close search tips' });
    await waitFor(() => expect(close).toHaveFocus());
    expect(close).toHaveClass('min-h-11', 'min-w-11', 'sm:min-h-8', 'sm:min-w-8');

    fireEvent.click(close);
    expect(screen.queryByRole('dialog', { name: 'Search tips' })).toBeNull();
    expect(trigger).toHaveFocus();
  });

  it('opens on hover when configured', async () => {
    render(() => (
      <SearchTipsPopover openOnHover tips={[{ code: 'tag:web', description: 'Filter by tag' }]} />
    ));

    const trigger = screen.getByRole('button', { name: 'Search tips' });
    fireEvent.mouseEnter(trigger.parentElement as HTMLElement);
    expect(await screen.findByRole('dialog', { name: 'Search tips' })).toBeInTheDocument();

    fireEvent.mouseLeave(trigger.parentElement as HTMLElement);
    expect(screen.queryByRole('dialog', { name: 'Search tips' })).toBeNull();
  });

  it('closes on Escape while open', async () => {
    render(() => (
      <SearchTipsPopover
        openOnHover
        tips={[{ code: 'cpu>80', description: 'Filter by CPU threshold' }]}
      />
    ));

    const trigger = screen.getByRole('button', { name: 'Search tips' });
    fireEvent.click(trigger);
    expect(await screen.findByRole('dialog', { name: 'Search tips' })).toBeInTheDocument();

    fireEvent.keyDown(window, { key: 'Escape' });
    expect(screen.queryByRole('dialog', { name: 'Search tips' })).toBeNull();
    expect(trigger).toHaveFocus();
  });

  it('keeps hover-enabled tips available while keyboard focus moves into them', async () => {
    render(() => (
      <>
        <SearchTipsPopover
          openOnHover
          tips={[{ code: 'name:web', description: 'Filter by name' }]}
        />
        <button type="button">After tips</button>
      </>
    ));

    const trigger = screen.getByRole('button', { name: 'Search tips' });
    const container = trigger.parentElement as HTMLElement;
    fireEvent.mouseEnter(container);
    const dialog = await screen.findByRole('dialog', { name: 'Search tips' });
    const close = screen.getByRole('button', { name: 'Close search tips' });

    trigger.focus();
    fireEvent.blur(trigger, { relatedTarget: close });
    close.focus();
    fireEvent.mouseLeave(container);
    expect(dialog).toBeInTheDocument();

    const after = screen.getByRole('button', { name: 'After tips' });
    fireEvent.blur(close, { relatedTarget: after });
    after.focus();
    expect(screen.queryByRole('dialog', { name: 'Search tips' })).toBeNull();
  });

  it('bounds landscape tips above the visible mobile navigation', async () => {
    vi.spyOn(window, 'innerWidth', 'get').mockReturnValue(844);
    vi.spyOn(window, 'innerHeight', 'get').mockReturnValue(390);

    render(() => (
      <>
        <nav aria-label="Mobile navigation" />
        <SearchTipsPopover tips={[{ code: 'name:web', description: 'Filter by name' }]} />
      </>
    ));

    const nav = screen.getByRole('navigation', { name: 'Mobile navigation' });
    vi.spyOn(nav, 'getBoundingClientRect').mockReturnValue(
      DOMRect.fromRect({ y: 328.5, height: 61.5, width: 844 }),
    );
    const trigger = screen.getByRole('button', { name: 'Search tips' });
    vi.spyOn(trigger, 'getBoundingClientRect').mockReturnValue(
      DOMRect.fromRect({ x: 752, y: 150, width: 28, height: 20 }),
    );

    fireEvent.click(trigger);
    const dialog = await screen.findByRole('dialog', { name: 'Search tips' });
    vi.spyOn(dialog, 'getBoundingClientRect').mockReturnValue(
      DOMRect.fromRect({ x: 492, y: 16, width: 288, height: 326 }),
    );
    window.dispatchEvent(new Event('resize'));

    await waitFor(() => {
      expect(dialog.style.position).toBe('fixed');
      expect(dialog.style.getPropertyPriority('position')).toBe('important');
      expect(dialog.style.top).toBe('16px');
      expect(dialog.style.maxHeight).toBe('296.5px');
    });
  });

  it('ignores the hidden mobile nav when bounding a short desktop window', async () => {
    vi.spyOn(window, 'innerWidth', 'get').mockReturnValue(1280);
    vi.spyOn(window, 'innerHeight', 'get').mockReturnValue(500);

    render(() => (
      <>
        <nav aria-label="Mobile navigation" />
        <SearchTipsPopover tips={[{ code: 'name:web', description: 'Filter by name' }]} />
      </>
    ));

    const nav = screen.getByRole('navigation', { name: 'Mobile navigation' });
    vi.spyOn(nav, 'getBoundingClientRect').mockReturnValue(DOMRect.fromRect({ height: 0 }));
    const trigger = screen.getByRole('button', { name: 'Search tips' });
    vi.spyOn(trigger, 'getBoundingClientRect').mockReturnValue(
      DOMRect.fromRect({ x: 1173, y: 200, width: 28, height: 20 }),
    );

    fireEvent.click(trigger);
    const dialog = await screen.findByRole('dialog', { name: 'Search tips' });
    vi.spyOn(dialog, 'getBoundingClientRect').mockReturnValue(
      DOMRect.fromRect({ x: 913, y: 16, width: 288, height: 326 }),
    );
    window.dispatchEvent(new Event('resize'));

    await waitFor(() => {
      expect(dialog.style.position).toBe('fixed');
      expect(dialog.style.maxHeight).toBe('468px');
    });
  });
});
