import { Component, For, Show } from 'solid-js';
import { SectionHeader } from '@/components/shared/SectionHeader';
import { Toggle } from '@/components/shared/Toggle';
import type { ToggleChangeEvent } from '@/components/shared/Toggle';
import {
  getDiscoverySubnetInvalidFormatMessage,
  getDiscoverySubnetRequiredMessage,
} from '@/utils/infrastructureSettingsPresentation';
import {
  getNetworkDiscoveryModePresentation,
  getNetworkDiscoveryPriorityNotice,
  getNetworkDiscoverySectionPresentation,
  getNetworkDiscoverySubnetPresentation,
} from '@/utils/discoveryPresentation';
import { COMMON_DISCOVERY_SUBNETS } from '@/utils/systemSettingsPresentation';
import type { NetworkDiscoverySectionProps } from './networkSettingsModel';

export const NetworkDiscoverySection: Component<NetworkDiscoverySectionProps> = (props) => {
  const priorityNotice = getNetworkDiscoveryPriorityNotice();
  const sectionPresentation = () =>
    getNetworkDiscoverySectionPresentation(props.discoveryEnabled());
  const autoModePresentation = getNetworkDiscoveryModePresentation('auto');
  const customModePresentation = getNetworkDiscoveryModePresentation('custom');
  const subnetPresentation = () => getNetworkDiscoverySubnetPresentation(props.discoveryMode());

  return (
    <>
      <div class="p-4 sm:p-6">
        <div class="rounded-md border border-blue-200 dark:border-blue-800 bg-blue-50/70 dark:bg-blue-950/40 p-4">
          <div class="flex items-start gap-3">
            <svg
              class="w-5 h-5 text-blue-600 dark:text-blue-400 mt-0.5 flex-shrink-0"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
              />
            </svg>
            <div class="text-sm text-blue-800 dark:text-blue-200">
              <p class="font-medium mb-1">{priorityNotice.title}</p>
              <ul class="space-y-1">
                <For each={priorityNotice.items}>{(item) => <li>• {item}</li>}</For>
              </ul>
            </div>
          </div>
        </div>
      </div>

      <section class="p-4 sm:p-6 space-y-5">
        <SectionHeader
          title={sectionPresentation().headerTitle}
          description={sectionPresentation().headerDescription}
          size="sm"
          align="left"
        />

        <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <div class="text-sm text-muted">
            <p class="font-medium text-base-content">{sectionPresentation().toggleTitle}</p>
            <p class="text-xs text-muted">{sectionPresentation().toggleDescription}</p>
          </div>
          <Toggle
            checked={props.discoveryEnabled()}
            onChange={async (e: ToggleChangeEvent) => {
              if (props.envOverrides().discoveryEnabled || props.savingDiscoverySettings()) {
                e.preventDefault();
                return;
              }
              const success = await props.handleDiscoveryEnabledChange(e.currentTarget.checked);
              if (!success) {
                e.currentTarget.checked = props.discoveryEnabled();
              }
            }}
            disabled={props.envOverrides().discoveryEnabled || props.savingDiscoverySettings()}
            containerClass="gap-2"
            label={
              <span class="text-xs font-medium text-muted">
                {sectionPresentation().toggleStateLabel}
              </span>
            }
          />
        </div>

        <Show when={props.discoveryEnabled()}>
          <div class="space-y-4 rounded-md border border-border bg-surface p-3">
            <fieldset class="space-y-2">
              <legend class="text-xs font-medium text-base-content">
                {sectionPresentation().scanScopeLabel}
              </legend>
              <div class="space-y-2">
                <label
                  class={`flex items-start gap-3 rounded-md border p-2 transition-colors ${
                    props.discoveryMode() === 'auto'
                      ? 'border-blue-200 bg-blue-50 dark:border-blue-700 dark:bg-blue-900'
                      : 'border-transparent hover:border-border'
                  }`}
                >
                  <input
                    type="radio"
                    name="discoveryMode"
                    value="auto"
                    checked={props.discoveryMode() === 'auto'}
                    onChange={async () => {
                      if (props.discoveryMode() !== 'auto') {
                        await props.handleDiscoveryModeChange('auto');
                      }
                    }}
                    disabled={props.envOverrides().discoverySubnet || props.savingDiscoverySettings()}
                    class="mt-1 h-5 w-5 sm:h-4 sm:w-4 border-slate-300 text-blue-600 focus:ring-blue-500"
                  />
                  <div class="space-y-1">
                    <p class="text-sm font-medium text-base-content">{autoModePresentation.label}</p>
                    <p class="text-xs text-muted">{autoModePresentation.description}</p>
                  </div>
                </label>

                <label
                  class={`flex items-start gap-3 rounded-md border p-2 transition-colors ${
                    props.discoveryMode() === 'custom'
                      ? 'border-blue-200 bg-blue-50 dark:border-blue-700 dark:bg-blue-900'
                      : 'border-transparent hover:border-border'
                  }`}
                >
                  <input
                    type="radio"
                    name="discoveryMode"
                    value="custom"
                    checked={props.discoveryMode() === 'custom'}
                    onChange={() => {
                      if (props.discoveryMode() !== 'custom') {
                        props.handleDiscoveryModeChange('custom');
                      }
                    }}
                    disabled={props.envOverrides().discoverySubnet || props.savingDiscoverySettings()}
                    class="mt-1 h-5 w-5 sm:h-4 sm:w-4 border-slate-300 text-blue-600 focus:ring-blue-500"
                  />
                  <div class="space-y-1">
                    <p class="text-sm font-medium text-base-content">
                      {customModePresentation.label}
                    </p>
                    <p class="text-xs text-muted">{customModePresentation.description}</p>
                  </div>
                </label>

                <Show when={props.discoveryMode() === 'custom'}>
                  <div class="flex flex-wrap items-center gap-2 pl-9 pr-2">
                    <span class="text-[0.68rem] uppercase tracking-wide text-muted">
                      {sectionPresentation().commonNetworksLabel}:
                    </span>
                    <For each={COMMON_DISCOVERY_SUBNETS}>
                      {(preset) => {
                        const baseValue = props.currentDraftSubnetValue();
                        const currentSelections = props.parseSubnetList(baseValue);
                        const isActive = currentSelections.includes(preset);
                        return (
                          <button
                            type="button"
                            class={`rounded border px-2.5 py-1 text-[0.7rem] transition-colors ${isActive ? 'border-blue-500 bg-blue-600 text-white dark:border-blue-400 dark:bg-blue-500' : 'border-border text-base-content hover:border-blue-400 hover:bg-blue-50 dark:hover:border-blue-500 dark:hover:bg-blue-900'}`}
                            onClick={async () => {
                              if (props.envOverrides().discoverySubnet) {
                                return;
                              }
                              let selections = [...currentSelections];
                              if (isActive) {
                                selections = selections.filter((item) => item !== preset);
                              } else {
                                selections.push(preset);
                              }

                              if (selections.length === 0) {
                                props.setDiscoverySubnetDraft('');
                                props.setLastCustomSubnet('');
                                props.setDiscoverySubnetError(getDiscoverySubnetRequiredMessage());
                                return;
                              }

                              const updatedValue = props.normalizeSubnetList(
                                selections.join(', '),
                              );
                              props.setDiscoveryMode('custom');
                              props.setDiscoverySubnetDraft(updatedValue);
                              props.setLastCustomSubnet(updatedValue);
                              props.setDiscoverySubnetError(undefined);
                              await props.commitDiscoverySubnet(updatedValue);
                            }}
                            disabled={props.envOverrides().discoverySubnet}
                            classList={{
                              'cursor-not-allowed opacity-60':
                                props.envOverrides().discoverySubnet,
                            }}
                          >
                            {preset}
                          </button>
                        );
                      }}
                    </For>
                  </div>
                </Show>
              </div>
            </fieldset>

            <div class="space-y-2">
              <div class="flex items-center justify-between gap-2">
                <label for="discoverySubnetInput" class="text-xs font-medium text-base-content">
                  {subnetPresentation().label}
                </label>
                <span
                  class="text-slate-400 hover:text-muted"
                  title={subnetPresentation().helpTooltip}
                >
                  <svg
                    class="h-5 w-5 sm:h-4 sm:w-4"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="2"
                  >
                    <circle cx="12" cy="12" r="10"></circle>
                    <path d="M12 16v-4"></path>
                    <path d="M12 8h.01"></path>
                  </svg>
                </span>
              </div>
              <input
                id="discoverySubnetInput"
                ref={props.discoverySubnetInputRef}
                type="text"
                value={props.discoverySubnetDraft()}
                placeholder={subnetPresentation().placeholder}
                class={`w-full min-h-10 sm:min-h-10 rounded-md border px-3 py-2.5 text-sm transition-colors focus:outline-none focus:ring-2 focus:ring-blue-500 ${
                  props.envOverrides().discoverySubnet
                    ? 'border-amber-300 bg-amber-50 text-amber-800 dark:border-amber-600 dark:bg-amber-900 dark:text-amber-200 cursor-not-allowed opacity-60'
                    : 'border-border bg-surface'
                }`}
                disabled={props.envOverrides().discoverySubnet}
                onInput={(e) => {
                  if (props.envOverrides().discoverySubnet) {
                    return;
                  }
                  const rawValue = e.currentTarget.value;
                  props.setDiscoverySubnetDraft(rawValue);
                  if (props.discoveryMode() !== 'custom') {
                    props.setDiscoveryMode('custom');
                  }
                  props.setLastCustomSubnet(rawValue);
                  const trimmed = rawValue.trim();
                  if (!trimmed) {
                    props.setDiscoverySubnetError(undefined);
                    return;
                  }
                  if (!props.isValidCIDR(trimmed)) {
                    props.setDiscoverySubnetError(getDiscoverySubnetInvalidFormatMessage());
                  } else {
                    props.setDiscoverySubnetError(undefined);
                  }
                }}
                onBlur={async (e) => {
                  if (props.envOverrides().discoverySubnet || props.discoveryMode() !== 'custom') {
                    return;
                  }
                  const rawValue = e.currentTarget.value;
                  props.setDiscoverySubnetDraft(rawValue);
                  const trimmed = rawValue.trim();
                  if (!trimmed) {
                    props.setDiscoverySubnetError(getDiscoverySubnetRequiredMessage());
                    return;
                  }
                  if (!props.isValidCIDR(trimmed)) {
                    props.setDiscoverySubnetError(getDiscoverySubnetInvalidFormatMessage());
                    return;
                  }
                  props.setDiscoverySubnetError(undefined);
                  await props.commitDiscoverySubnet(rawValue);
                }}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') {
                    e.preventDefault();
                    (e.currentTarget as HTMLInputElement).blur();
                  }
                }}
              />
              <Show when={props.discoverySubnetError()}>
                <p class="text-xs text-red-600 dark:text-red-400">{props.discoverySubnetError()}</p>
              </Show>
              <Show when={!props.discoverySubnetError() && props.discoveryMode() === 'auto'}>
                <p class="text-xs text-muted">{subnetPresentation().guidance}</p>
              </Show>
              <Show when={!props.discoverySubnetError() && props.discoveryMode() === 'custom'}>
                <p class="text-xs text-muted">{subnetPresentation().guidance}</p>
              </Show>
            </div>
          </div>
        </Show>

        <Show when={props.envOverrides().discoveryEnabled || props.envOverrides().discoverySubnet}>
          <div class="rounded-md border border-amber-200 bg-amber-100 p-3 text-xs text-amber-800 dark:border-amber-700 dark:bg-amber-900 dark:text-amber-200">
            {sectionPresentation().environmentOverrideMessage}
          </div>
        </Show>
      </section>
    </>
  );
};
