import { afterEach, describe, expect, it } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { SearchTipsPopover } from '@/components/shared/SearchTipsPopover';
import searchTipsPopoverSource from '@/components/shared/SearchTipsPopover.tsx?raw';
import searchTipsPopoverModelSource from '@/components/shared/searchTipsPopoverModel.ts?raw';
import searchTipsPopoverStateSource from '@/components/shared/useSearchTipsPopoverState.ts?raw';

describe('SearchTipsPopover', () => {
  afterEach(() => {
    cleanup();
  });

  it('keeps search tips popover on shell, runtime, and model owners', () => {
    expect(searchTipsPopoverSource).toContain('useSearchTipsPopoverState');
    expect(searchTipsPopoverSource).toContain('getSearchTipsPopoverTriggerClass');
    expect(searchTipsPopoverSource).not.toContain('createSignal');
    expect(searchTipsPopoverSource).not.toContain('createEffect');
    expect(searchTipsPopoverSource).not.toContain('window.addEventListener');
    expect(searchTipsPopoverSource).not.toContain('triggerVariant ===');

    expect(searchTipsPopoverStateSource).toContain('export function useSearchTipsPopoverState');
    expect(searchTipsPopoverStateSource).toContain('createSignal');
    expect(searchTipsPopoverStateSource).toContain('createEffect');
    expect(searchTipsPopoverStateSource).toContain('window.addEventListener');
    expect(searchTipsPopoverStateSource).toContain('pointerInside');

    expect(searchTipsPopoverModelSource).toContain('getSearchTipsPopoverTriggerClass');
    expect(searchTipsPopoverModelSource).toContain('getSearchTipsPopoverPositionClass');
    expect(searchTipsPopoverModelSource).toContain('getSearchTipsPopoverTriggerVariant');
    expect(searchTipsPopoverModelSource).toContain('shouldSearchTipsPopoverOpenOnHover');
  });

  it('toggles the popover on click by default', async () => {
    render(() => (
      <SearchTipsPopover
        tips={[{ code: 'name:web', description: 'Filter by name' }]}
      />
    ));

    const trigger = screen.getByRole('button', { name: 'Search tips' });
    expect(screen.queryByRole('dialog', { name: 'Search tips' })).toBeNull();

    fireEvent.click(trigger);
    expect(await screen.findByRole('dialog', { name: 'Search tips' })).toBeInTheDocument();

    fireEvent.click(trigger);
    expect(screen.queryByRole('dialog', { name: 'Search tips' })).toBeNull();
  });

  it('opens on hover when configured', async () => {
    render(() => (
      <SearchTipsPopover
        openOnHover
        tips={[{ code: 'tag:web', description: 'Filter by tag' }]}
      />
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
        tips={[{ code: 'cpu>80', description: 'Filter by CPU threshold' }]}
      />
    ));

    fireEvent.click(screen.getByRole('button', { name: 'Search tips' }));
    expect(await screen.findByRole('dialog', { name: 'Search tips' })).toBeInTheDocument();

    fireEvent.keyDown(window, { key: 'Escape' });
    expect(screen.queryByRole('dialog', { name: 'Search tips' })).toBeNull();
  });
});
