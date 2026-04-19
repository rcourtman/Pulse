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

const DEFAULT_MANUAL_TYPES: ConnectionType[] = ['pve', 'pbs', 'pmg', 'truenas', 'vmware', 'agent'];

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
                Paste an address and Pulse detects the product. One flow for every supported
                platform.
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
                  Choose type manually
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
