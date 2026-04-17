import { Component, For } from 'solid-js';
import { Dialog } from '@/components/shared/Dialog';
import type { ConnectionKind, ConnectionMethod } from './connectionsTableModel';

export interface AddSystemChoice {
  kind: ConnectionKind;
  title: string;
  description: string;
  method: ConnectionMethod;
  methodLabel: string;
}

export const ADD_SYSTEM_CHOICES: readonly AddSystemChoice[] = [
  {
    kind: 'pve',
    title: 'Proxmox VE',
    description: 'Connect a Proxmox cluster or standalone node. Pulse polls the Proxmox API.',
    method: 'api',
    methodLabel: 'API connection',
  },
  {
    kind: 'pbs',
    title: 'Proxmox Backup Server',
    description: 'Connect a PBS instance so Pulse can track datastores, sync, and verify jobs.',
    method: 'api',
    methodLabel: 'API connection',
  },
  {
    kind: 'pmg',
    title: 'Proxmox Mail Gateway',
    description: 'Connect a PMG instance for queue, quarantine, and mail-stats reporting.',
    method: 'api',
    methodLabel: 'API connection',
  },
  {
    kind: 'truenas',
    title: 'TrueNAS SCALE',
    description: 'Connect a TrueNAS system so Pulse can report pools, datasets, and apps.',
    method: 'api',
    methodLabel: 'API connection',
  },
  {
    kind: 'vmware',
    title: 'VMware vSphere or ESXi',
    description: 'Connect a vCenter or ESXi host. Pulse polls the vSphere API.',
    method: 'api',
    methodLabel: 'API connection',
  },
  {
    kind: 'agent',
    title: 'Linux or Docker host (agent)',
    description:
      'Generate an install token and run the Pulse agent on the host itself. Best for anything without a supported API.',
    method: 'agent',
    methodLabel: 'Agent install',
  },
];

interface AddSystemPickerProps {
  isOpen: boolean;
  onClose: () => void;
  onSelect: (choice: AddSystemChoice) => void;
}

export const AddSystemPicker: Component<AddSystemPickerProps> = (props) => {
  return (
    <Dialog
      isOpen={props.isOpen}
      onClose={props.onClose}
      ariaLabelledBy="add-system-picker-title"
      closeOnBackdrop
      panelClass="w-full max-w-2xl rounded-lg bg-surface shadow-xl"
    >
      <div class="space-y-4 p-6">
        <div class="flex items-start justify-between gap-4">
          <div>
            <h2 id="add-system-picker-title" class="text-lg font-semibold text-base-content">
              Add a system
            </h2>
            <p class="mt-1 text-sm text-muted">
              Pick what you are connecting. Pulse will route you straight into the right flow.
            </p>
          </div>
          <button
            type="button"
            onClick={props.onClose}
            class="rounded-md border border-border bg-surface px-2 py-1 text-xs text-muted hover:bg-surface-hover"
            aria-label="Close"
          >
            Close
          </button>
        </div>
        <ul class="divide-y divide-border rounded-md border border-border">
          <For each={ADD_SYSTEM_CHOICES}>
            {(choice) => (
              <li>
                <button
                  type="button"
                  onClick={() => props.onSelect(choice)}
                  class="flex w-full items-start justify-between gap-4 px-4 py-3 text-left hover:bg-surface-hover"
                >
                  <div>
                    <div class="text-sm font-semibold text-base-content">{choice.title}</div>
                    <div class="mt-0.5 text-xs text-muted">{choice.description}</div>
                  </div>
                  <span
                    class={`inline-flex shrink-0 items-center rounded-full px-2 py-0.5 text-xs font-medium ${
                      choice.method === 'api'
                        ? 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-100'
                        : 'bg-emerald-100 text-emerald-800 dark:bg-emerald-900 dark:text-emerald-100'
                    }`}
                  >
                    {choice.methodLabel}
                  </span>
                </button>
              </li>
            )}
          </For>
        </ul>
      </div>
    </Dialog>
  );
};
