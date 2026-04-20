import { Component, For, Show, createSignal } from 'solid-js';
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
    // Ignore storage failures (private mode, quota); the in-memory signal is enough for this session.
  }
};

const ALWAYS_ON_CAPABILITIES = ['Hardware telemetry'];

const OPT_IN_CAPABILITIES = ['Assistant commands', 'Patrol remediation'];

const AGENT_FACTS = [
  'Single Go binary',
  '~13 MB download',
  'No runtime dependencies',
  'Open source',
];

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
        class="relative overflow-hidden rounded-lg border border-border bg-surface"
      >
        <button
          type="button"
          onClick={handleDismiss}
          aria-label="Dismiss"
          class="absolute right-2 top-2 z-10 inline-flex items-center justify-center rounded-md p-1 text-muted transition-colors hover:bg-surface-hover hover:text-base-content"
        >
          <X class="h-4 w-4" aria-hidden="true" />
        </button>

        <div class="border-b border-border px-5 py-3">
          <h3 class="text-sm font-semibold text-base-content">
            How Pulse collects data
          </h3>
          <p class="mt-0.5 text-xs text-muted">
            The platform API covers workloads. The Unified Agent adds host-level
            telemetry, and stands alone where there's no API.
          </p>
        </div>

        <div class="grid grid-cols-1 divide-border md:grid-cols-2 md:divide-x">
          <div class="p-5">
            <div class="flex items-start gap-3">
              <div
                aria-hidden="true"
                class="flex h-9 w-9 flex-none items-center justify-center rounded-lg border border-border bg-surface-alt text-base-content"
              >
                <Cloud class="h-4 w-4" />
              </div>
              <div class="min-w-0">
                <div class="text-sm font-semibold text-base-content">Platform API</div>
                <div class="text-xs text-muted">Primary source for workloads</div>
              </div>
            </div>
            <p class="mt-3 text-xs leading-relaxed text-muted">
              Pulse polls the platform's own API (Proxmox VE / PBS / PMG, VMware,
              TrueNAS) for VMs, containers, storage, backups, and other workload data.
              Required for every API-backed target; fastest to set up.
            </p>
          </div>

          <div class="relative bg-blue-50/40 p-5 dark:bg-blue-950/20">
            <div
              aria-hidden="true"
              class="absolute inset-y-0 left-0 w-0.5 bg-blue-500 md:hidden"
            />
            <div class="flex items-start gap-3">
              <div
                aria-hidden="true"
                class="flex h-9 w-9 flex-none items-center justify-center rounded-lg border border-blue-200 bg-blue-100 text-blue-700 dark:border-blue-900 dark:bg-blue-900/40 dark:text-blue-300"
              >
                <Cpu class="h-4 w-4" />
              </div>
              <div class="min-w-0">
                <div class="text-sm font-semibold text-base-content">
                  Pulse Unified Agent
                </div>
                <div class="text-xs text-muted">Runs on the host, alongside the API</div>
              </div>
            </div>

            <p class="mt-3 text-xs leading-relaxed text-muted">
              On Proxmox / VMware / TrueNAS, the agent supplements the API with data
              it can't expose (CPU and disk temperatures, SMART, power, Ceph/RAID).
              On bare-metal Linux, Unraid, or FreeBSD with no platform API, it's the
              only path.
            </p>

            <div class="mt-4 space-y-2">
              <div>
                <div class="mb-1 text-[10px] font-semibold uppercase tracking-wide text-muted">
                  Always on
                </div>
                <div class="flex flex-wrap gap-1.5">
                  <For each={ALWAYS_ON_CAPABILITIES}>
                    {(label) => (
                      <span class="inline-flex items-center rounded-full border border-border bg-surface px-2 py-0.5 text-[11px] font-medium text-base-content">
                        {label}
                      </span>
                    )}
                  </For>
                </div>
              </div>
              <div>
                <div class="mb-1 text-[10px] font-semibold uppercase tracking-wide text-muted">
                  Off by default, opt in per host
                </div>
                <div class="flex flex-wrap gap-1.5">
                  <For each={OPT_IN_CAPABILITIES}>
                    {(label) => (
                      <span class="inline-flex items-center rounded-full border border-dashed border-border bg-surface px-2 py-0.5 text-[11px] font-medium text-muted">
                        {label}
                      </span>
                    )}
                  </For>
                </div>
              </div>
            </div>

            <ul class="mt-4 flex flex-wrap gap-x-3 gap-y-1 text-[11px] text-muted">
              <For each={AGENT_FACTS}>
                {(fact, index) => (
                  <li class="flex items-center gap-1.5">
                    <Show when={index() > 0}>
                      <span aria-hidden="true" class="text-border">
                        ·
                      </span>
                    </Show>
                    {fact}
                  </li>
                )}
              </For>
            </ul>
          </div>
        </div>
      </section>
    </Show>
  );
};
