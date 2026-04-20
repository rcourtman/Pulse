import { Component, type JSX, Show, createMemo, createSignal } from 'solid-js';
import type { ConnectionType, ProbeCandidate } from '@/api/connections';
import { AddressProbeStep } from './AddressProbeStep';
import {
  CONNECTION_TYPE_LABELS,
  createConnectionEditorState,
  type ConnectionEditorState,
} from './useConnectionEditor';

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
  renderCredentialSlot: CredentialSlotRenderer;
  manualTypeOptions?: ConnectionType[];
  onClose: () => void;
  onSaved?: () => void;
}

const DEFAULT_MANUAL_TYPES: ConnectionType[] = ['pve', 'pbs', 'pmg', 'truenas', 'vmware'];

export const ConnectionEditor: Component<ConnectionEditorProps> = (props) => {
  const state: ConnectionEditorState = createConnectionEditorState();
  if (props.initialAddress) {
    state.setAddress(props.initialAddress);
  }

  const [selectedType, setSelectedType] = createSignal<ConnectionType | null>(
    props.initialType ?? null,
  );
  const [selectedCandidate, setSelectedCandidate] = createSignal<ProbeCandidate | null>(null);
  const [manualPickerOpen, setManualPickerOpen] = createSignal(false);

  const manualOptions = createMemo(() => props.manualTypeOptions ?? DEFAULT_MANUAL_TYPES);

  const activeType = () => selectedType();
  const showCredentialSlot = () => activeType() !== null;

  const chooseCandidate = (candidate: ProbeCandidate) => {
    setSelectedCandidate(candidate);
    setSelectedType(candidate.type);
    setManualPickerOpen(false);
  };

  const chooseManualType = (type: ConnectionType) => {
    setSelectedCandidate(null);
    setSelectedType(type);
    setManualPickerOpen(false);
  };

  const reopenProbe = () => {
    setSelectedCandidate(null);
    setSelectedType(null);
    setManualPickerOpen(false);
  };

  const handleSaved = () => {
    props.onSaved?.();
    props.onClose();
  };

  return (
    <div class="flex h-full flex-col">
      <Show
        when={showCredentialSlot()}
        fallback={
          <div class="space-y-4 p-4">
            <div>
              <div class="text-sm font-semibold text-base-content">Add a connection</div>
              <div class="mt-0.5 text-xs text-muted">
                Paste a platform address to connect its API, or install the Unified Agent on a
                host.
              </div>
            </div>

            <section class="space-y-3 rounded-md border border-border bg-surface-alt/30 p-3">
              <div>
                <div class="text-xs font-semibold uppercase tracking-wide text-muted">
                  Platform API
                </div>
                <div class="text-[11px] text-muted">
                  Proxmox VE / PBS / PMG, VMware, TrueNAS
                </div>
              </div>

              <AddressProbeStep
                state={state}
                onSelectCandidate={chooseCandidate}
                onChooseManually={() => setManualPickerOpen((v) => !v)}
              />

              <Show when={manualPickerOpen()}>
                <div class="space-y-2 rounded-md border border-border bg-surface p-3">
                  <div class="text-xs font-semibold uppercase tracking-wide text-muted">
                    Choose Platform API type manually
                  </div>
                  <ul class="divide-y divide-border rounded-md border border-border">
                    {manualOptions().map((type) => (
                      <li>
                        <button
                          type="button"
                          class="flex w-full items-center justify-between px-3 py-2 text-left text-sm text-base-content transition-colors hover:bg-surface-hover"
                          onClick={() => chooseManualType(type)}
                        >
                          <span>{CONNECTION_TYPE_LABELS[type] ?? type}</span>
                          <span class="text-xs text-muted">{type}</span>
                        </button>
                      </li>
                    ))}
                  </ul>
                </div>
              </Show>
            </section>

            <section class="space-y-3 rounded-md border border-blue-200 bg-blue-50/40 p-3 dark:border-blue-900 dark:bg-blue-950/20">
              <div>
                <div class="text-xs font-semibold uppercase tracking-wide text-muted">
                  Pulse Unified Agent
                </div>
                <div class="text-[11px] text-muted">
                  Host-level telemetry on Proxmox / VMware / TrueNAS, or the only path on
                  bare-metal Linux, Unraid, FreeBSD.
                </div>
              </div>
              <button
                type="button"
                onClick={() => chooseManualType('agent')}
                class="inline-flex items-center rounded-md border border-blue-600 bg-blue-600 px-3 py-2 text-sm font-medium text-white transition-colors hover:bg-blue-500"
              >
                Install the Unified Agent on a host
              </button>
            </section>
          </div>
        }
      >
        <div class="flex items-center justify-between border-b border-border px-4 py-2">
          <div class="text-sm">
            <span class="font-semibold text-base-content">
              {CONNECTION_TYPE_LABELS[activeType()!] ?? activeType()}
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
              ← Back to probe
            </button>
          </Show>
        </div>

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
