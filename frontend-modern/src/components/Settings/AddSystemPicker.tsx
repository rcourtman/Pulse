import { Component, For } from 'solid-js';
import { Dialog } from '@/components/shared/Dialog';

export type AddSystemChoiceKind = 'pve' | 'pbs' | 'pmg' | 'truenas' | 'vmware' | 'agent';
export type AddSystemChoiceMethod = 'api' | 'agent';

export interface AddSystemChoice {
  kind: AddSystemChoiceKind;
  title: string;
  description: string;
  method: AddSystemChoiceMethod;
  methodLabel: string;
}

export const ADD_SYSTEM_CHOICES: readonly AddSystemChoice[] = [
  {
    kind: 'agent',
    title: 'Install on a host',
    description:
      'Generate an install token and run the Pulse agent on the host itself. Best for the first monitored system or anything without a supported API.',
    method: 'agent',
    methodLabel: 'Agent install',
  },
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
];

interface AddSystemPickerProps {
  isOpen: boolean;
  onClose: () => void;
  onSelect: (choice: AddSystemChoice) => void;
  onManageProfiles?: () => void;
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
              Add infrastructure
            </h2>
            <p class="mt-1 text-sm text-muted">
              Choose the system or platform you want Pulse to start monitoring.
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
        <For each={props.onManageProfiles ? [props.onManageProfiles] : []}>
          {(onManageProfiles) => (
            <div class="flex items-center justify-between gap-3 rounded-md border border-border bg-surface-alt px-4 py-3">
              <div class="text-xs text-muted">
                Agent profiles change install defaults for agent-based systems.
              </div>
              <button
                type="button"
                onClick={onManageProfiles}
                class="inline-flex items-center rounded-md border border-border px-3 py-1.5 text-sm font-medium text-base-content transition-colors hover:bg-surface-hover"
              >
                Manage agent profiles
              </button>
            </div>
          )}
        </For>
      </div>
    </Dialog>
  );
};
