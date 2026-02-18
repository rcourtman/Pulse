import { For, createMemo } from 'solid-js';
import { Dialog } from '@/components/shared/Dialog';

interface ShortcutGroup {
  title: string;
  items: { keys: string; description: string }[];
}

interface KeyboardShortcutsModalProps {
  isOpen: boolean;
  onClose: () => void;
}

const UNIFIED_NAV_SHORTCUTS: ShortcutGroup = {
  title: 'Navigation',
  items: [
    { keys: 'g then i', description: 'Go to Infrastructure' },
    { keys: 'g then w', description: 'Go to Workloads' },
    { keys: 'g then s', description: 'Go to Storage' },
    { keys: 'g then b', description: 'Go to Recovery' },
    { keys: 'g then a', description: 'Go to Alerts' },
    { keys: 'g then t', description: 'Go to Settings' },
  ],
};

const CLASSIC_NAV_SHORTCUTS: ShortcutGroup = {
  title: 'Migration Shortcuts (Old Muscle Memory)',
  items: [
    { keys: 'g then p', description: 'Go to Proxmox (filtered Infrastructure)' },
    { keys: 'g then h', description: 'Go to Hosts (filtered Infrastructure)' },
    { keys: 'g then d', description: 'Go to Container Hosts (filtered Infrastructure)' },
    { keys: 'g then v', description: 'Go to Services (filtered Infrastructure)' },
    { keys: 'g then c', description: 'Go to Containers (filtered Workloads)' },
    { keys: 'g then l', description: 'Go to LXC Containers (filtered Workloads)' },
    { keys: 'g then k', description: 'Go to Kubernetes (filtered Workloads)' },
  ],
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
    return [UNIFIED_NAV_SHORTCUTS, CLASSIC_NAV_SHORTCUTS, SEARCH_SHORTCUTS];
  });

  return (
    <Dialog
      isOpen={props.isOpen}
      onClose={props.onClose}
      panelClass="max-w-xl"
      ariaLabel="Keyboard shortcuts"
    >
      <div class="flex items-center justify-between border-b border-gray-200 px-5 py-4 dark:border-gray-700">
        <h2 class="text-lg font-semibold text-gray-900 dark:text-gray-100">
          Keyboard Shortcuts
        </h2>
        <button
          type="button"
          onClick={props.onClose}
          class="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
          aria-label="Close shortcuts"
        >
          <svg class="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>

      <div class="flex-1 overflow-y-auto space-y-5 px-5 py-4">
        <For each={shortcutGroups()}>
          {(group) => (
            <div>
              <div class="text-[11px] font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
                {group.title}
              </div>
              <div class="mt-2 space-y-2">
                <For each={group.items}>
                  {(item) => (
                    <div class="flex items-center justify-between text-sm text-gray-700 dark:text-gray-300">
                      <span>{item.description}</span>
                      <span class="rounded bg-gray-100 px-2 py-1 text-xs font-medium text-gray-700 dark:bg-gray-700 dark:text-gray-200">
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

      <div class="border-t border-gray-200 px-5 py-3 text-xs text-gray-500 dark:border-gray-700 dark:text-gray-400">
        Press <span class="font-medium">?</span> again or <span class="font-medium">Esc</span> to close.
      </div>
    </Dialog>
  );
}

export default KeyboardShortcutsModal;
