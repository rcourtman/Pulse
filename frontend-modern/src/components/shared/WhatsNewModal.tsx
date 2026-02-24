import { createEffect, createSignal, Show } from 'solid-js';
import { createLocalStorageBooleanSignal, STORAGE_KEYS } from '@/utils/localStorage';
import ServerIcon from 'lucide-solid/icons/server';
import BoxesIcon from 'lucide-solid/icons/boxes';
import HardDriveIcon from 'lucide-solid/icons/hard-drive';
import ShieldCheckIcon from 'lucide-solid/icons/shield-check';
import ChartBarIcon from 'lucide-solid/icons/chart-bar';
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
      <div class="fixed inset-0 z-50 flex items-center justify-center bg-black p-4">
        <div class="w-full max-w-2xl max-h-[90vh] flex flex-col overflow-hidden rounded-md bg-surface shadow-sm">
          <div class="flex-shrink-0 flex items-start justify-between border-b border-border px-6 py-4">
            <div>
              <h2 class="text-xl sm:text-2xl font-semibold text-base-content">
                Welcome to the New Navigation!
              </h2>
              <p class="mt-1 text-sm text-muted">
                Everything is now organized by what you want to do, not where the data comes from.
              </p>
            </div>
            <button
              onClick={handleClose}
              class="rounded-md p-1.5 text-slate-400 transition-colors hover:bg-surface-hover hover:text-muted"
              aria-label="Close"
            >
              <XIcon class="h-5 w-5" />
            </button>
          </div>

          <div class="flex-1 overflow-y-auto space-y-4 sm:space-y-6 px-4 sm:px-6 py-4 sm:py-5">
            <div class="grid gap-3 sm:gap-4 sm:grid-cols-2">
              <div class="rounded-md border border-blue-200 bg-blue-50 p-3 sm:p-4 dark:border-blue-800 dark:bg-blue-900">
                <div class="flex items-center gap-2 text-sm font-semibold text-blue-900 dark:text-blue-100">
                  <ServerIcon class="h-4 w-4" />
                  Infrastructure
                </div>
                <p class="mt-1.5 sm:mt-2 text-xs text-blue-900 dark:text-blue-100">
                  Proxmox nodes, Hosts, and container hosts live together in one unified view.
                </p>
              </div>

              <div class="rounded-md border border-purple-200 bg-purple-50 p-3 sm:p-4 dark:border-purple-800 dark:bg-purple-900">
                <div class="flex items-center gap-2 text-sm font-semibold text-purple-900 dark:text-purple-100">
                  <BoxesIcon class="h-4 w-4" />
                  Workloads
                </div>
                <p class="mt-1.5 sm:mt-2 text-xs text-purple-900 dark:text-purple-100">
                  All VMs, containers, and Kubernetes workloads now share a single list.
                </p>
              </div>

              <div class="rounded-md border border-emerald-200 bg-emerald-50 p-3 sm:p-4 dark:border-emerald-800 dark:bg-emerald-900">
                <div class="flex items-center gap-2 text-sm font-semibold text-emerald-900 dark:text-emerald-100">
                  <HardDriveIcon class="h-4 w-4" />
                  Storage
                </div>
                <p class="mt-1.5 sm:mt-2 text-xs text-emerald-900 dark:text-emerald-100">
                  Storage is now a top-level destination across all systems.
                </p>
              </div>

              <div class="rounded-md border border-amber-200 bg-amber-50 p-3 sm:p-4 dark:border-amber-800 dark:bg-amber-900">
                <div class="flex items-center gap-2 text-sm font-semibold text-amber-900 dark:text-amber-100">
                  <ShieldCheckIcon class="h-4 w-4" />
                  Recovery
                </div>
                <p class="mt-1.5 sm:mt-2 text-xs text-amber-900 dark:text-amber-100">
                  Recovery events (backups, snapshots, and replication) are now first-class pages.
                </p>
              </div>
            </div>

            <div class="rounded-md border border-sky-200 bg-sky-50 p-3 sm:p-4 dark:border-sky-800 dark:bg-sky-900/40">
              <div class="flex items-center gap-2 text-sm font-medium text-sky-900 dark:text-sky-100">
                <ChartBarIcon class="h-4 w-4 flex-shrink-0" />
                Anonymous telemetry
              </div>
              <p class="mt-1.5 text-xs text-sky-900 dark:text-sky-200">
                Pulse now sends a lightweight anonymous ping once a day — just a random install ID,
                version, platform, resource counts, and feature flags. No hostnames, credentials, IP
                addresses, or personal information is ever sent.
              </p>
              <p class="mt-1.5 text-xs text-sky-900 dark:text-sky-200">
                This helps the developer understand how Pulse is used and prioritise what to build
                next. You can disable it any time in{' '}
                <span class="font-medium">Settings &rarr; System &rarr; General</span> or by setting{' '}
                <code class="rounded bg-sky-100 px-1 py-0.5 text-[10px] font-mono dark:bg-sky-800">
                  PULSE_TELEMETRY=false
                </code>
                .{' '}
                <a
                  href="https://github.com/rcourtman/Pulse/blob/main/docs/PRIVACY.md"
                  target="_blank"
                  rel="noopener noreferrer"
                  class="underline hover:text-sky-700 dark:hover:text-sky-100"
                >
                  Full details
                </a>
              </p>
            </div>

            <div class="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3 sm:gap-4">
              <label class="flex items-center gap-2 text-sm text-muted">
                <input
                  type="checkbox"
                  checked={dontShowAgain()}
                  onChange={(event) => setDontShowAgain(event.currentTarget.checked)}
                  class="h-4 w-4 rounded border-slate-300 text-blue-600 focus:ring-blue-500 focus:ring-2"
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
              <a
                href="/migration-guide"
                class="inline-flex items-center gap-1 text-sm font-medium text-blue-600 hover:text-blue-700 dark:text-blue-400 dark:hover:text-blue-300"
              >
                Migration guide
              </a>
            </div>
          </div>

          <div class="flex-shrink-0 flex items-center justify-end border-t border-border bg-surface-hover px-4 sm:px-6 py-3 sm:py-4">
            <button
              onClick={handleClose}
              class="rounded-md bg-blue-600 px-4 py-2 text-sm font-semibold text-white transition-colors hover:bg-blue-700"
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
