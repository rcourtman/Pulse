import { Component, For, type JSX, Show, createEffect, createMemo, createSignal, onMount } from 'solid-js';
import type { ConnectionType, ProbeCandidate } from '@/api/connections';
import { AddressProbeStep } from './AddressProbeStep';
import {
  CONNECTION_TYPE_LABELS,
  createConnectionEditorState,
  type ConnectionEditorState,
} from './useConnectionEditor';
import { getInfrastructureAutoDetectLabels } from '@/utils/infrastructureOnboardingPresentation';
import {
  createInfrastructureOnboardingMetricsTracker,
  type InfrastructureOnboardingMetricsTracker,
} from '@/utils/infrastructureOnboardingMetrics';

export type ConnectionEditorMode = 'add' | 'edit';

export interface ConnectionEditorSlotContext {
  mode: ConnectionEditorMode;
  type: ConnectionType;
  candidate: ProbeCandidate | null;
  onCancel: () => void;
  onSaved: () => void;
}

export type CredentialSlotRenderer = (context: ConnectionEditorSlotContext) => JSX.Element;

export interface ConnectionEditorProps {
  mode?: ConnectionEditorMode;
  initialType?: ConnectionType;
  initialAddress?: string;
  initialCandidate?: ProbeCandidate | null;
  showSlotHeader?: boolean;
  trackInitialCatalogSelection?: boolean;
  onboardingMetricsTracker?: InfrastructureOnboardingMetricsTracker | null;
  onBackToCatalog?: () => void;
  onSelectAgentRoute?: () => void;
  onSelectCandidate?: (candidate: ProbeCandidate) => void;
  renderCredentialSlot: CredentialSlotRenderer;
  onClose: () => void;
  onSaved?: () => void;
}

export const ConnectionEditor: Component<ConnectionEditorProps> = (props) => {
  const state: ConnectionEditorState = createConnectionEditorState();
  if (props.initialAddress) {
    state.setAddress(props.initialAddress);
  }

  const [selectedType, setSelectedType] = createSignal<ConnectionType | null>(
    props.initialType ?? null,
  );
  const [selectedCandidate, setSelectedCandidate] = createSignal<ProbeCandidate | null>(
    props.initialCandidate ?? null,
  );
  const ownsOnboardingMetricsTracker =
    (props.mode ?? 'add') === 'add' && !props.onboardingMetricsTracker;
  const onboardingMetrics =
    (props.mode ?? 'add') === 'add'
      ? props.onboardingMetricsTracker ?? createInfrastructureOnboardingMetricsTracker()
      : null;

  const activeType = () => selectedType();
  const showCredentialSlot = () => activeType() !== null;
  const autoDetectLabels = createMemo(() => getInfrastructureAutoDetectLabels());

  const recordPathSelectedForType = (type: ConnectionType) => {
    onboardingMetrics?.recordPathSelected(type === 'agent' ? 'agent' : 'api');
  };

  onMount(() => {
    if (!onboardingMetrics) return;
    if (ownsOnboardingMetricsTracker) {
      onboardingMetrics.recordOpened();
    }
    if (props.initialType) {
      recordPathSelectedForType(props.initialType);
      if (props.trackInitialCatalogSelection && props.initialType !== 'agent') {
        onboardingMetrics.recordCatalogSelected(props.initialType);
      }
    }
  });

  createEffect(() => {
    const type = selectedType();
    if (!onboardingMetrics || !type) return;
    onboardingMetrics.recordCredentialsOpened(type);
  });

  const chooseCandidate = (candidate: ProbeCandidate) => {
    onboardingMetrics?.recordPathSelected('api');
    if (props.onSelectCandidate) {
      props.onSelectCandidate(candidate);
      return;
    }
    setSelectedCandidate(candidate);
    setSelectedType(candidate.type);
  };

  const chooseManualType = (type: ConnectionType) => {
    recordPathSelectedForType(type);
    setSelectedCandidate(null);
    setSelectedType(type);
  };

  const installAgent = () => {
    if (props.onSelectAgentRoute) {
      onboardingMetrics?.recordPathSelected('agent');
      props.onSelectAgentRoute();
      return;
    }
    chooseManualType('agent');
  };

  const reopenProbe = () => {
    state.reset();
    setSelectedCandidate(null);
    setSelectedType(null);
  };

  const handleSaved = () => {
    props.onSaved?.();
    props.onClose();
  };

  const renderBadge = (label: string) => (
    <span class="inline-flex items-center rounded-full border border-border bg-surface px-2 py-0.5 text-[11px] font-medium text-base-content">
      {label}
    </span>
  );

  return (
    <div class="flex h-full min-h-0 flex-col">
      <Show
        when={showCredentialSlot()}
        fallback={
          <div class="space-y-6 p-4">
            <section class="space-y-4 rounded-xl border border-border bg-surface p-4">
              <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                <div class="space-y-1">
                  <div class="text-sm font-semibold text-base-content">Address probe</div>
                  <p class="text-xs text-muted">
                    Pulse can auto-detect these platforms from an address when their management API
                    is reachable.
                  </p>
                </div>
                <Show when={props.onBackToCatalog}>
                  <button
                    type="button"
                    onClick={props.onBackToCatalog}
                    class="inline-flex items-center self-start rounded-md border border-border px-3 py-2 text-sm font-medium text-base-content transition-colors hover:bg-surface-hover"
                  >
                    ← Back to source types
                  </button>
                </Show>
              </div>

              <div class="flex flex-wrap gap-1.5">
                <For each={autoDetectLabels()}>{(label) => renderBadge(label)}</For>
              </div>

              <p class="text-xs text-muted">
                Not in this list?{' '}
                <button
                  type="button"
                  onClick={installAgent}
                  class="font-medium text-blue-600 underline underline-offset-2 hover:text-blue-500 dark:text-blue-300 dark:hover:text-blue-200"
                >
                  Install Pulse Agent
                </button>{' '}
                for Linux, macOS, Windows, FreeBSD, or Unraid hosts.
              </p>

              <AddressProbeStep
                state={state}
                onSelectCandidate={chooseCandidate}
                onInstallAgent={installAgent}
                onChooseSourceTypeInstead={props.onBackToCatalog}
                onProbeSubmitted={() => onboardingMetrics?.recordPathSelected('api')}
                onProbeResolved={(outcome) => onboardingMetrics?.recordProbeResult(outcome)}
              />
            </section>
          </div>
        }
      >
        <Show when={props.showSlotHeader ?? true}>
          <div class="flex items-center justify-between border-b border-border px-4 py-2">
            <div class="text-sm">
              <span class="font-semibold text-base-content">
                {activeType() === 'agent'
                  ? 'Install Pulse Agent'
                  : (CONNECTION_TYPE_LABELS[activeType()!] ?? activeType())}
              </span>
              <Show when={selectedCandidate()}>
                <span class="ml-2 text-xs text-muted">{selectedCandidate()!.host}</span>
              </Show>
            </div>
            <Show when={(props.mode ?? 'add') === 'add'}>
              <button
                type="button"
                onClick={reopenProbe}
                class="inline-flex items-center rounded-md border border-border px-2.5 py-1 text-xs font-medium text-base-content transition-colors hover:bg-surface-hover"
              >
                ← Back to detect
              </button>
            </Show>
          </div>
        </Show>

        <Show
          when={
            !(props.showSlotHeader ?? true) &&
            (props.mode ?? 'add') === 'add' &&
            props.onBackToCatalog
          }
        >
          <div class="border-b border-border bg-surface-alt px-4 py-2">
            <button
              type="button"
              onClick={props.onBackToCatalog}
              class="inline-flex items-center rounded-md border border-border px-2.5 py-1 text-xs font-medium text-base-content transition-colors hover:bg-surface-hover"
            >
              ← Back to source types
            </button>
          </div>
        </Show>

        <div class="flex-1 overflow-y-auto p-4">
          {props.renderCredentialSlot({
            mode: props.mode ?? 'add',
            type: activeType()!,
            candidate: selectedCandidate(),
            onCancel: props.onClose,
            onSaved: handleSaved,
          })}
        </div>
      </Show>
    </div>
  );
};
