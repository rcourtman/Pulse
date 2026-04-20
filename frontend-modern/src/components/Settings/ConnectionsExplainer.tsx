import { Component, Show, createSignal } from 'solid-js';
import { Cloud, Cpu, X } from 'lucide-solid';

const DISMISS_KEY = 'pulse.infrastructure.explainer.dismissed.v1';

const readDismissed = (): boolean => {
  if (typeof window === 'undefined') return false;
  try {
    return window.localStorage.getItem(DISMISS_KEY) === '1';
  } catch {
    return false;
  }
};

const persistDismissed = () => {
  if (typeof window === 'undefined') return;
  try {
    window.localStorage.setItem(DISMISS_KEY, '1');
  } catch {
    // Ignore storage failures (private mode, quota) — the in-memory signal is enough for this session.
  }
};

export const ConnectionsExplainer: Component = () => {
  const [dismissed, setDismissed] = createSignal(readDismissed());

  const handleDismiss = () => {
    persistDismissed();
    setDismissed(true);
  };

  return (
    <Show when={!dismissed()}>
      <section
        aria-label="How to connect infrastructure"
        class="relative rounded-md border border-border bg-surface-alt px-4 py-4"
      >
      <button
        type="button"
        onClick={handleDismiss}
        aria-label="Dismiss"
        class="absolute right-2 top-2 inline-flex items-center justify-center rounded-md p-1 text-muted transition-colors hover:bg-surface-hover hover:text-base-content"
      >
        <X class="h-4 w-4" aria-hidden="true" />
      </button>

      <div class="mb-3 pr-8">
        <h3 class="text-sm font-semibold text-base-content">
          Two ways to connect infrastructure
        </h3>
        <p class="text-xs text-muted">
          Pick whichever fits the target. You can mix both on the same host.
        </p>
      </div>

      <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
        <div class="flex gap-3">
          <div
            aria-hidden="true"
            class="mt-0.5 flex h-8 w-8 flex-none items-center justify-center rounded-md border border-border bg-surface text-muted"
          >
            <Cloud class="h-4 w-4" />
          </div>
          <div class="min-w-0 space-y-1">
            <div class="text-sm font-medium text-base-content">Platform API</div>
            <p class="text-xs text-muted">
              Pulse polls the platform's own API (Proxmox VE / PBS / PMG, VMware,
              TrueNAS). Fastest to set up; coverage matches what the platform exposes.
            </p>
          </div>
        </div>

        <div class="flex gap-3">
          <div
            aria-hidden="true"
            class="mt-0.5 flex h-8 w-8 flex-none items-center justify-center rounded-md border border-border bg-surface text-muted"
          >
            <Cpu class="h-4 w-4" />
          </div>
          <div class="min-w-0 space-y-1">
            <div class="text-sm font-medium text-base-content">Pulse Unified Agent</div>
            <p class="text-xs text-muted">
              Installs on the host itself. Use when there's no API (bare-metal Linux,
              Unraid), when you want data the API can't surface (CPU/disk temps, SMART,
              power, Ceph/RAID), or to let Assistant and Patrol run commands and fixes
              on the host.
            </p>
          </div>
        </div>
      </div>
      </section>
    </Show>
  );
};
