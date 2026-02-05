import { createEffect, createSignal, Show } from 'solid-js';
import { createLocalStorageBooleanSignal, STORAGE_KEYS } from '@/utils/localStorage';
import ServerIcon from 'lucide-solid/icons/server';
import BoxesIcon from 'lucide-solid/icons/boxes';
import HardDriveIcon from 'lucide-solid/icons/hard-drive';
import ShieldCheckIcon from 'lucide-solid/icons/shield-check';
import ExternalLinkIcon from 'lucide-solid/icons/external-link';
import XIcon from 'lucide-solid/icons/x';

const DOCS_URL = 'https://github.com/rcourtman/Pulse/blob/main/docs/README.md';

export function WhatsNewModal() {
  const [hasSeen, setHasSeen] = createLocalStorageBooleanSignal(
    STORAGE_KEYS.WHATS_NEW_NAV_V2_SHOWN,
    false,
  );
  const [isOpen, setIsOpen] = createSignal(false);
  const [dontShowAgain, setDontShowAgain] = createSignal(true);

  createEffect(() => {
    if (!hasSeen()) {
      setIsOpen(true);
    }
  });

  const handleClose = () => {
    if (dontShowAgain() || hasSeen()) {
      setHasSeen(true);
    }
    setIsOpen(false);
  };

  return (
    <Show when={isOpen()}>
      <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
        <div class="w-full max-w-2xl overflow-hidden rounded-2xl bg-white shadow-2xl dark:bg-gray-800">
          <div class="flex items-start justify-between border-b border-gray-200 px-6 py-4 dark:border-gray-700">
            <div>
              <h2 class="text-2xl font-semibold text-gray-900 dark:text-gray-100">
                Welcome to the New Navigation!
              </h2>
              <p class="mt-1 text-sm text-gray-600 dark:text-gray-400">
                Everything is now organized by what you want to do, not where the data comes from.
              </p>
            </div>
            <button
              onClick={handleClose}
              class="rounded-lg p-1.5 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-gray-700 dark:hover:text-gray-300"
              aria-label="Close"
            >
              <XIcon class="h-5 w-5" />
            </button>
          </div>

          <div class="space-y-6 px-6 py-5">
            <div class="grid gap-4 sm:grid-cols-2">
              <div class="rounded-xl border border-blue-200 bg-blue-50/70 p-4 dark:border-blue-800/60 dark:bg-blue-900/20">
                <div class="flex items-center gap-2 text-sm font-semibold text-blue-900 dark:text-blue-100">
                  <ServerIcon class="h-4 w-4" />
                  Infrastructure
                </div>
                <p class="mt-2 text-xs text-blue-900/80 dark:text-blue-100/80">
                  Proxmox nodes, Hosts, and Docker hosts live together in one unified view.
                </p>
              </div>

              <div class="rounded-xl border border-purple-200 bg-purple-50/70 p-4 dark:border-purple-800/60 dark:bg-purple-900/20">
                <div class="flex items-center gap-2 text-sm font-semibold text-purple-900 dark:text-purple-100">
                  <BoxesIcon class="h-4 w-4" />
                  Workloads
                </div>
                <p class="mt-2 text-xs text-purple-900/80 dark:text-purple-100/80">
                  All VMs, containers, and Docker workloads now share a single list.
                </p>
              </div>

              <div class="rounded-xl border border-emerald-200 bg-emerald-50/70 p-4 dark:border-emerald-800/60 dark:bg-emerald-900/20">
                <div class="flex items-center gap-2 text-sm font-semibold text-emerald-900 dark:text-emerald-100">
                  <HardDriveIcon class="h-4 w-4" />
                  Storage
                </div>
                <p class="mt-2 text-xs text-emerald-900/80 dark:text-emerald-100/80">
                  Storage is now a top-level destination across all systems.
                </p>
              </div>

              <div class="rounded-xl border border-amber-200 bg-amber-50/70 p-4 dark:border-amber-800/60 dark:bg-amber-900/20">
                <div class="flex items-center gap-2 text-sm font-semibold text-amber-900 dark:text-amber-100">
                  <ShieldCheckIcon class="h-4 w-4" />
                  Backups
                </div>
                <p class="mt-2 text-xs text-amber-900/80 dark:text-amber-100/80">
                  Backup status and replication are now first-class pages.
                </p>
              </div>
            </div>

            <div class="rounded-xl border border-gray-200 bg-gray-50 p-4 text-sm text-gray-700 dark:border-gray-700 dark:bg-gray-900/40 dark:text-gray-300">
              <div class="font-medium text-gray-900 dark:text-gray-100">
                Quick summary
              </div>
              <ul class="mt-2 space-y-2">
                <li class="flex items-start gap-2">
                  <span class="mt-1 h-1.5 w-1.5 rounded-full bg-blue-500"></span>
                  <span>Infrastructure combines Proxmox nodes, Hosts, and Docker hosts.</span>
                </li>
                <li class="flex items-start gap-2">
                  <span class="mt-1 h-1.5 w-1.5 rounded-full bg-purple-500"></span>
                  <span>Workloads now shows every VM, container, and Docker container.</span>
                </li>
                <li class="flex items-start gap-2">
                  <span class="mt-1 h-1.5 w-1.5 rounded-full bg-amber-500"></span>
                  <span>Storage and Backups live at the top level for faster access.</span>
                </li>
              </ul>
            </div>

            <div class="flex items-center justify-between gap-4">
              <label class="flex items-center gap-2 text-sm text-gray-600 dark:text-gray-400">
                <input
                  type="checkbox"
                  checked={dontShowAgain()}
                  onChange={(event) => setDontShowAgain(event.currentTarget.checked)}
                  class="h-4 w-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500 focus:ring-2"
                />
                Don&#39;t show again
              </label>

              <a
                href={DOCS_URL}
                target="_blank"
                rel="noopener noreferrer"
                class="inline-flex items-center gap-1 text-sm font-medium text-blue-600 hover:text-blue-700 dark:text-blue-400 dark:hover:text-blue-300"
              >
                Documentation
                <ExternalLinkIcon class="h-4 w-4" />
              </a>
            </div>
          </div>

          <div class="flex items-center justify-end border-t border-gray-200 bg-gray-50 px-6 py-4 dark:border-gray-700 dark:bg-gray-900/50">
            <button
              onClick={handleClose}
              class="rounded-lg bg-blue-600 px-4 py-2 text-sm font-semibold text-white transition-colors hover:bg-blue-700"
            >
              Let&#39;s go
            </button>
          </div>
        </div>
      </div>
    </Show>
  );
}

export default WhatsNewModal;
