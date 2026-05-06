import { Component, For, Show } from 'solid-js';
import { Activity, Archive, Cpu, Database, Mail, Search, Server, ServerCog } from 'lucide-solid';
import type { InfrastructureOnboardingConnectionType } from '@/utils/infrastructureOnboardingPresentation';
import {
  getInfrastructureSourcePickerGroups,
  getInfrastructureOnboardingProductPresentation,
  getInfrastructureSourceStrategyPresentation,
} from '@/utils/infrastructureOnboardingPresentation';

interface InfrastructureSourcePickerProps {
  onSelectType: (type: InfrastructureOnboardingConnectionType) => void;
  onDetectFromAddress?: () => void;
}

const detectButtonClass =
  'inline-flex items-center rounded-md border border-border px-3 py-2 text-sm font-medium text-base-content transition-colors hover:bg-surface-hover';

const CARD_ICON: Record<InfrastructureOnboardingConnectionType, Component<{ class?: string }>> = {
  vmware: ServerCog,
  truenas: Database,
  pve: Server,
  pbs: Archive,
  pmg: Mail,
  agent: Cpu,
  availability: Activity,
};

export const InfrastructureSourcePicker: Component<InfrastructureSourcePickerProps> = (props) => {
  const groups = () => getInfrastructureSourcePickerGroups();

  return (
    <div class="space-y-6 p-4">
      <Show when={props.onDetectFromAddress}>
        <div class="flex flex-col gap-3 rounded-md border border-border bg-surface-alt p-3 sm:flex-row sm:items-center sm:justify-between">
          <div class="space-y-1">
            <div class="text-sm font-medium text-base-content">Not sure which source to use?</div>
            <p class="text-xs text-muted">
              Probe an address and Pulse will open the matching API inventory flow when it
              recognizes the platform.
            </p>
          </div>
          <button
            type="button"
            onClick={props.onDetectFromAddress}
            class={`${detectButtonClass} self-start sm:self-center`}
          >
            <Search class="mr-2 h-4 w-4" />
            Detect from address
          </button>
        </div>
      </Show>

      <For each={groups()}>
        {(group) => (
          <section class="space-y-3">
            <div class="space-y-1">
              <h3 class="text-sm font-semibold text-base-content">{group.label}</h3>
              <p class="text-xs text-muted">{group.description}</p>
            </div>

            <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
              <For each={group.products}>
                {(product) => {
                  const Icon = CARD_ICON[product.type];
                  const details = getInfrastructureOnboardingProductPresentation(product.type);
                  const strategy = getInfrastructureSourceStrategyPresentation(
                    details.sourceStrategy,
                  );
                  return (
                    <button
                      type="button"
                      onClick={() => props.onSelectType(product.type)}
                      class="group flex h-full flex-col gap-3 rounded-md border border-border bg-surface p-4 text-left transition-colors hover:border-blue-500 hover:bg-blue-50/40 dark:hover:bg-blue-950/20"
                    >
                      <div class="flex items-start gap-3">
                        <div
                          aria-hidden="true"
                          class="flex h-10 w-10 flex-none items-center justify-center rounded-md border border-border bg-surface-alt text-base-content"
                        >
                          <Icon class="h-5 w-5" />
                        </div>
                        <div class="min-w-0 space-y-1">
                          <div class="flex flex-wrap items-center gap-2">
                            <div class="text-sm font-semibold text-base-content">
                              {details.label}
                            </div>
                            <span class="inline-flex items-center rounded-full border border-border bg-surface-alt px-2 py-0.5 text-[10px] font-medium uppercase tracking-wide text-base-content">
                              {strategy.label}
                            </span>
                            <Show when={details.governanceState === 'admitted'}>
                              <span class="inline-flex items-center rounded-full border border-blue-200 bg-blue-100 px-2 py-0.5 text-[10px] font-medium uppercase tracking-wide text-blue-800 dark:border-blue-900 dark:bg-blue-950/40 dark:text-blue-200">
                                Available now
                              </span>
                            </Show>
                          </div>
                          <div class="text-xs text-muted">{details.catalogDescription}</div>
                        </div>
                      </div>

                      <p class="text-[11px] text-muted">{strategy.detail}</p>

                      <div class="grid gap-2 text-[11px] text-muted">
                        <div>
                          <div class="font-medium uppercase tracking-wide text-base-content">
                            Best for
                          </div>
                          <div>{details.bestFor}</div>
                        </div>
                        <div>
                          <div class="font-medium uppercase tracking-wide text-base-content">
                            Unlocks
                          </div>
                          <div>{details.coverage}</div>
                        </div>
                      </div>
                    </button>
                  );
                }}
              </For>
            </div>
          </section>
        )}
      </For>
    </div>
  );
};

export default InfrastructureSourcePicker;
