import { createSignal, onCleanup, type Accessor } from 'solid-js';
import { useNavigate } from '@solidjs/router';
import {
  buildAvailabilityPath,
  buildDockerPath,
  buildKubernetesPath,
  buildProxmoxPath,
  buildStandalonePath,
  buildTrueNASPath,
  buildVmwarePath,
} from '@/routing/resourceLinks';
import {
  createEmptyPlatformNavigationVisibility,
  filterPlatformNavigationShortcuts,
  type PlatformNavigationVisibility,
} from '@/features/platformNavigation/platformNavigationModel';
import { focusActiveTypeToSearch, registerTypeToSearchKeyGuard } from '@/hooks/useTypeToSearch';

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
  platformVisibility?: Accessor<PlatformNavigationVisibility>;
  // Patrol navigation exists only while AI is enabled and configured; an
  // absent accessor keeps the shortcut for callers that do not gate it.
  patrolVisible?: Accessor<boolean>;
};

const isEditableTarget = (target: EventTarget | null): boolean => {
  if (!target || !(target instanceof HTMLElement)) return false;
  const tag = target.tagName.toLowerCase();
  if (tag === 'input' || tag === 'textarea' || tag === 'select') return true;
  if (target.isContentEditable) return true;
  if (target.getAttribute('role') === 'textbox') return true;
  return false;
};

const isModifierOnlyKey = (key: string): boolean =>
  key === 'Shift' || key === 'Control' || key === 'Alt' || key === 'Meta';

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
    focusActiveTypeToSearch({ selectText: true });
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

  const getRoutes = (): Record<string, string> => {
    const platformVisibility =
      options.platformVisibility?.() ?? createEmptyPlatformNavigationVisibility();
    const infrastructureRoutes = filterPlatformNavigationShortcuts(
      {
        proxmox: { key: 'p', route: buildProxmoxPath() },
        docker: { key: 'd', route: buildDockerPath() },
        kubernetes: { key: 'k', route: buildKubernetesPath() },
        truenas: { key: 'n', route: buildTrueNASPath() },
        vmware: { key: 'v', route: buildVmwarePath() },
        standalone: { key: 's', route: buildStandalonePath() },
        availability: { key: 'c', route: buildAvailabilityPath() },
      },
      platformVisibility,
    );
    return {
      ...infrastructureRoutes,
      a: '/alerts',
      ...(options.patrolVisible?.() === false ? {} : { r: '/patrol' }),
      t: '/settings',
    };
  };

  const shouldReserveKeyForShortcuts = (e: KeyboardEvent): boolean => {
    if (options.enabled && !options.enabled()) return false;
    const shortcutsOpen = options.isShortcutsOpen?.() ?? false;
    const paletteOpen = options.isCommandPaletteOpen?.() ?? false;
    if (shortcutsOpen || paletteOpen) return false;
    if (isEditableTarget(e.target) || isEditableTarget(document.activeElement)) return false;
    if (e.metaKey || e.ctrlKey || e.altKey) return false;
    if (awaitingSecondKey()) return true;
    const key = e.key.toLowerCase();
    return (key === 'g' && !e.shiftKey) || key === '/' || e.key === '?';
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

    const key = e.key.toLowerCase();

    if ((e.metaKey || e.ctrlKey) && key === 'k') {
      e.preventDefault();
      clearAwaiting();
      if (paletteOpen) {
        if (options.onToggleCommandPalette) {
          options.onToggleCommandPalette();
        } else {
          options.onCloseCommandPalette?.();
        }
      } else {
        openCommandPalette();
      }
      return;
    }

    if (shortcutsOpen) {
      if (e.key === '?' && !e.metaKey && !e.ctrlKey && !e.altKey && !isEditableTarget(e.target)) {
        e.preventDefault();
        if (options.onToggleShortcuts) {
          options.onToggleShortcuts();
        } else {
          options.onCloseShortcuts?.();
        }
      }
      return;
    }

    if (paletteOpen) {
      return;
    }

    if (isEditableTarget(e.target)) {
      return;
    }

    if (awaitingSecondKey()) {
      if (isModifierOnlyKey(e.key)) {
        return;
      }
      clearAwaiting();
      if (!e.metaKey && !e.ctrlKey && !e.altKey) {
        e.preventDefault();
        const route = getRoutes()[key];
        if (route) {
          navigate(route);
        }
      }
      return;
    }

    if (key === 'g' && !e.shiftKey && !e.metaKey && !e.ctrlKey && !e.altKey) {
      e.preventDefault();
      if (!e.repeat) {
        startAwaiting();
      }
      return;
    }

    if (key === '/' && !e.metaKey && !e.ctrlKey && !e.altKey) {
      e.preventDefault();
      focusSearch();
      return;
    }

    if (e.key === '?' && !e.metaKey && !e.ctrlKey && !e.altKey) {
      e.preventDefault();
      openShortcuts();
    }
  };

  if (typeof document !== 'undefined') {
    const unregisterKeyGuard = registerTypeToSearchKeyGuard(shouldReserveKeyForShortcuts);
    document.addEventListener('keydown', handleKeyDown);
    onCleanup(() => {
      unregisterKeyGuard();
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
