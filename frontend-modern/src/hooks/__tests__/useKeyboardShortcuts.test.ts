import { createRoot, createSignal } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { useTypeToSearch } from '@/hooks/useTypeToSearch';

const navigateMock = vi.fn();

vi.mock('@solidjs/router', () => ({
  useNavigate: () => navigateMock,
}));

import { useKeyboardShortcuts } from '@/hooks/useKeyboardShortcuts';

describe('useKeyboardShortcuts', () => {
  let disposers: Array<() => void> = [];

  afterEach(() => {
    disposers.forEach((dispose) => dispose());
    disposers = [];
    navigateMock.mockReset();
    document.body.replaceChildren();
    vi.useRealTimers();
  });

  it.each(['shortcuts-first', 'type-to-search-first'] as const)(
    'navigates on g then a without type-to-search hijacking the input (%s)',
    async (order) => {
      const searchInput = document.createElement('input');
      document.body.append(searchInput);

      const mountTypeToSearch = () =>
        createRoot((dispose) => {
          disposers.push(dispose);
          useTypeToSearch({ getInput: () => searchInput });
        });

      const mountShortcuts = () =>
        createRoot((dispose) => {
          disposers.push(dispose);
          useKeyboardShortcuts({
            enabled: () => true,
            patrolVisible: () => true,
          });
        });

      if (order === 'shortcuts-first') {
        mountShortcuts();
        mountTypeToSearch();
      } else {
        mountTypeToSearch();
        mountShortcuts();
      }

      const gEvent = new KeyboardEvent('keydown', { key: 'g', bubbles: true, cancelable: true });
      document.body.dispatchEvent(gEvent);
      await Promise.resolve();

      expect(gEvent.defaultPrevented).toBe(true);
      expect(searchInput.value).toBe('');
      expect(document.activeElement).not.toBe(searchInput);

      const aEvent = new KeyboardEvent('keydown', { key: 'a', bubbles: true, cancelable: true });
      document.body.dispatchEvent(aEvent);
      await Promise.resolve();

      expect(aEvent.defaultPrevented).toBe(true);
      expect(navigateMock).toHaveBeenCalledExactlyOnceWith('/alerts');
      expect(searchInput.value).toBe('');
    },
  );

  it('toggles the shortcuts modal open and closed on repeated ? presses', () => {
    const [shortcutsOpen, setShortcutsOpen] = createSignal(false);
    createRoot((dispose) => {
      disposers.push(dispose);
      useKeyboardShortcuts({
        isShortcutsOpen: shortcutsOpen,
        onToggleShortcuts: () => setShortcutsOpen((open) => !open),
        onCloseShortcuts: () => setShortcutsOpen(false),
      });
    });

    const openEvent = new KeyboardEvent('keydown', {
      key: '?',
      shiftKey: true,
      bubbles: true,
      cancelable: true,
    });
    document.body.dispatchEvent(openEvent);
    expect(openEvent.defaultPrevented).toBe(true);
    expect(shortcutsOpen()).toBe(true);

    const closeEvent = new KeyboardEvent('keydown', {
      key: '?',
      shiftKey: true,
      bubbles: true,
      cancelable: true,
    });
    document.body.dispatchEvent(closeEvent);
    expect(closeEvent.defaultPrevented).toBe(true);
    expect(shortcutsOpen()).toBe(false);
  });

  it('toggles the command palette open and closed on Ctrl+K / Meta+K even from an input', () => {
    const [paletteOpen, setPaletteOpen] = createSignal(false);
    const paletteInput = document.createElement('input');
    document.body.append(paletteInput);

    createRoot((dispose) => {
      disposers.push(dispose);
      useKeyboardShortcuts({
        isCommandPaletteOpen: paletteOpen,
        onToggleCommandPalette: () => setPaletteOpen((open) => !open),
        onCloseCommandPalette: () => setPaletteOpen(false),
      });
    });

    const openEvent = new KeyboardEvent('keydown', {
      key: 'k',
      ctrlKey: true,
      bubbles: true,
      cancelable: true,
    });
    document.body.dispatchEvent(openEvent);
    expect(openEvent.defaultPrevented).toBe(true);
    expect(paletteOpen()).toBe(true);

    paletteInput.focus();
    const closeEvent = new KeyboardEvent('keydown', {
      key: 'k',
      ctrlKey: true,
      bubbles: true,
      cancelable: true,
    });
    paletteInput.dispatchEvent(closeEvent);
    expect(closeEvent.defaultPrevented).toBe(true);
    expect(paletteOpen()).toBe(false);
  });

  it('does not swallow ? while typing in the command palette input', () => {
    const [shortcutsOpen, setShortcutsOpen] = createSignal(false);
    const paletteInput = document.createElement('input');
    document.body.append(paletteInput);

    createRoot((dispose) => {
      disposers.push(dispose);
      useKeyboardShortcuts({
        isShortcutsOpen: shortcutsOpen,
        isCommandPaletteOpen: () => true,
        onToggleShortcuts: () => setShortcutsOpen((open) => !open),
      });
    });

    paletteInput.focus();
    const event = new KeyboardEvent('keydown', {
      key: '?',
      shiftKey: true,
      bubbles: true,
      cancelable: true,
    });
    paletteInput.dispatchEvent(event);
    expect(event.defaultPrevented).toBe(false);
    expect(shortcutsOpen()).toBe(false);
  });
});
