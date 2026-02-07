import { createSignal, onCleanup, type Accessor } from 'solid-js';
import { useNavigate } from '@solidjs/router';
import {
  buildBackupsPath,
  buildInfrastructurePath,
  buildStoragePath,
  buildWorkloadsPath,
} from '@/routing/resourceLinks';

type KeyboardShortcutsOptions = {
  enabled?: Accessor<boolean>;
  isShortcutsOpen?: Accessor<boolean>;
  isCommandPaletteOpen?: Accessor<boolean>;
  onOpenShortcuts?: () => void;
  onCloseShortcuts?: () => void;
  onToggleShortcuts?: () => void;
  onOpenCommandPalette?: () => void;
  onCloseCommandPalette?: () => void;
  onToggleCommandPalette?: () => void;
  onFocusSearch?: () => boolean | void;
};

const isEditableTarget = (target: EventTarget | null): boolean => {
  if (!target || !(target instanceof HTMLElement)) return false;
  const tag = target.tagName.toLowerCase();
  if (tag === 'input' || tag === 'textarea' || tag === 'select') return true;
  if (target.isContentEditable) return true;
  if (target.getAttribute('role') === 'textbox') return true;
  return false;
};

export function useKeyboardShortcuts(options: KeyboardShortcutsOptions = {}) {
  const navigate = useNavigate();
  const [awaitingSecondKey, setAwaitingSecondKey] = createSignal(false);
  let awaitingTimeout: number | undefined;

  const clearAwaiting = () => {
    if (awaitingTimeout !== undefined) {
      window.clearTimeout(awaitingTimeout);
      awaitingTimeout = undefined;
    }
    setAwaitingSecondKey(false);
  };

  const startAwaiting = () => {
    clearAwaiting();
    setAwaitingSecondKey(true);
    awaitingTimeout = window.setTimeout(() => {
      setAwaitingSecondKey(false);
      awaitingTimeout = undefined;
    }, 1000);
  };

  const focusSearch = () => {
    const handled = options.onFocusSearch?.();
    if (handled) return;
    const el = document.querySelector<HTMLInputElement>('[data-global-search]');
    if (el) {
      el.focus();
      el.select?.();
      return;
    }
    const trigger = document.querySelector<HTMLElement>('[data-global-search-trigger]');
    if (trigger) {
      trigger.click();
    }
  };

  const openShortcuts = () => {
    if (options.onToggleShortcuts) {
      options.onToggleShortcuts();
      return;
    }
    options.onOpenShortcuts?.();
  };

  const openCommandPalette = () => {
    if (options.onToggleCommandPalette) {
      options.onToggleCommandPalette();
      return;
    }
    options.onOpenCommandPalette?.();
  };

  const routes: Record<string, string> = {
    i: buildInfrastructurePath(),
    w: buildWorkloadsPath(),
    s: buildStoragePath(),
    b: buildBackupsPath(),
    a: '/alerts',
    t: '/settings',
  };

  const handleKeyDown = (e: KeyboardEvent) => {
    if (options.enabled && !options.enabled()) {
      return;
    }

    const shortcutsOpen = options.isShortcutsOpen?.() ?? false;
    const paletteOpen = options.isCommandPaletteOpen?.() ?? false;

    if (e.key === 'Escape') {
      if (awaitingSecondKey()) {
        clearAwaiting();
      }
      if (shortcutsOpen) {
        options.onCloseShortcuts?.();
      }
      if (paletteOpen) {
        options.onCloseCommandPalette?.();
      }
      return;
    }

    if (shortcutsOpen || paletteOpen) {
      return;
    }

    if (isEditableTarget(e.target)) {
      return;
    }

    const key = e.key.toLowerCase();

    if (key === 'g' && !awaitingSecondKey() && !e.metaKey && !e.ctrlKey && !e.altKey) {
      if (!e.repeat) {
        startAwaiting();
      }
      return;
    }

    if (awaitingSecondKey()) {
      clearAwaiting();
      const route = routes[key];
      if (route) {
        e.preventDefault();
        navigate(route);
      }
      return;
    }

    if (key === '/' && !e.metaKey && !e.ctrlKey && !e.altKey) {
      e.preventDefault();
      focusSearch();
      return;
    }

    if ((e.metaKey || e.ctrlKey) && key === 'k') {
      e.preventDefault();
      openCommandPalette();
      return;
    }

    if (e.key === '?') {
      e.preventDefault();
      openShortcuts();
    }
  };

  if (typeof document !== 'undefined') {
    document.addEventListener('keydown', handleKeyDown);
    onCleanup(() => {
      document.removeEventListener('keydown', handleKeyDown);
      if (awaitingTimeout !== undefined) {
        window.clearTimeout(awaitingTimeout);
      }
    });
  }

  return {
    awaitingSecondKey,
  };
}

export default useKeyboardShortcuts;
