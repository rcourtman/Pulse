import { For, createMemo } from 'solid-js';
import { Dialog } from '@/components/shared/Dialog';
import {
  primaryInfrastructureNavigationIsVisible,
  type InfrastructureNavigationVisibility,
} from '@/features/infrastructureNavigation/infrastructureNavigationModel';

interface ShortcutGroup {
  title: string;
  items: { keys: string; description: string }[];
}

interface KeyboardShortcutsModalProps {
  isOpen: boolean;
  onClose: () => void;
  infrastructureVisibility: () => InfrastructureNavigationVisibility;
}

const UNIFIED_NAV_SHORTCUTS: ShortcutGroup = {
  title: 'Navigation',
  items: [
    { keys: 'g then s', description: 'Go to Machines' },
    { keys: 'g then p', description: 'Go to Proxmox' },
    { keys: 'g then d', description: 'Go to Containers' },
    { keys: 'g then k', description: 'Go to Kubernetes' },
    { keys: 'g then n', description: 'Go to TrueNAS' },
    { keys: 'g then v', description: 'Go to vSphere' },
    { keys: 'g then a', description: 'Go to Alerts' },
    { keys: 'g then r', description: 'Go to Patrol' },
    { keys: 'g then t', description: 'Go to Settings' },
  ],
};

const NAV_PRIMARY_SHORTCUTS: Record<string, keyof InfrastructureNavigationVisibility> = {
  'g then s': 'standalone',
  'g then p': 'proxmox',
  'g then d': 'docker',
  'g then k': 'kubernetes',
  'g then n': 'truenas',
  'g then v': 'vmware',
};

const SEARCH_SHORTCUTS: ShortcutGroup = {
  title: 'Search & Help',
  items: [
    { keys: '/', description: 'Focus search' },
    { keys: 'Cmd+K / Ctrl+K', description: 'Open command palette' },
    { keys: '?', description: 'Show keyboard shortcuts' },
    { keys: 'Esc', description: 'Close dialogs / cancel' },
  ],
};

export function KeyboardShortcutsModal(props: KeyboardShortcutsModalProps) {
  const shortcutGroups = createMemo<ShortcutGroup[]>(() => {
    const infrastructureVisibility = props.infrastructureVisibility();
    const visibleNavigationItems = UNIFIED_NAV_SHORTCUTS.items.filter((item) => {
      const navId = NAV_PRIMARY_SHORTCUTS[item.keys];
      if (!navId) return true;
      return primaryInfrastructureNavigationIsVisible(infrastructureVisibility, navId);
    });
    return [{ ...UNIFIED_NAV_SHORTCUTS, items: visibleNavigationItems }, SEARCH_SHORTCUTS];
  });

  return (
    <Dialog
      isOpen={props.isOpen}
      onClose={props.onClose}
      panelClass="max-w-xl"
      ariaLabel="Keyboard shortcuts"
    >
      <div class="flex items-center justify-between border-b border-border px-5 py-4">
        <h2 class="text-lg font-semibold text-base-content">Keyboard Shortcuts</h2>
        <button
          type="button"
          onClick={props.onClose}
          class="text-slate-400 hover:text-muted"
          aria-label="Close shortcuts"
        >
          <svg class="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M6 18L18 6M6 6l12 12"
            />
          </svg>
        </button>
      </div>

      <div class="flex-1 overflow-y-auto space-y-5 px-5 py-4">
        <For each={shortcutGroups()}>
          {(group) => (
            <div>
              <div class="text-[11px] font-semibold uppercase tracking-wide text-muted">
                {group.title}
              </div>
              <div class="mt-2 space-y-2">
                <For each={group.items}>
                  {(item) => (
                    <div class="flex items-center justify-between text-sm text-base-content">
                      <span>{item.description}</span>
                      <span class="rounded bg-surface-alt px-2 py-1 text-xs font-medium text-base-content">
                        {item.keys}
                      </span>
                    </div>
                  )}
                </For>
              </div>
            </div>
          )}
        </For>
      </div>

      <div class="border-t border-border px-5 py-3 text-xs text-muted">
        Press <span class="font-medium">?</span> again or <span class="font-medium">Esc</span> to
        close.
      </div>
    </Dialog>
  );
}

export default KeyboardShortcutsModal;
