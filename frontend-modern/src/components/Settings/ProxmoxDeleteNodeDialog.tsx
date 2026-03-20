import type { Component } from 'solid-js';
import { Show } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { Dialog } from '@/components/shared/Dialog';
import { SectionHeader } from '@/components/shared/SectionHeader';

interface ProxmoxDeleteNodeDialogProps {
  deleteNodeLoading: boolean;
  nodePendingDeleteHost: string;
  nodePendingDeleteLabel: string;
  nodePendingDeleteType: string;
  nodePendingDeleteTypeLabel: string;
  onCancel: () => void;
  onDelete: () => Promise<void>;
}

export const ProxmoxDeleteNodeDialog: Component<ProxmoxDeleteNodeDialogProps> = (props) => {
  return (
    <Dialog
      isOpen={true}
      onClose={props.onCancel}
      panelClass="max-w-lg"
      closeOnBackdrop={false}
      ariaLabel={`Remove ${props.nodePendingDeleteLabel}`}
    >
      <Card padding="lg" class="w-full max-w-lg space-y-5">
        <SectionHeader title={`Remove ${props.nodePendingDeleteLabel}`} size="md" class="mb-1" />
        <div class="space-y-3 text-sm text-gray-600">
          <p>
            Removing this {props.nodePendingDeleteTypeLabel.toLowerCase()} also scrubs the Pulse
            footprint on the host - the proxy service, SSH key, API token, and bind mount are all
            cleaned up automatically.
          </p>
          <div class="rounded-md border border-blue-200 bg-blue-50 p-3 text-sm leading-relaxed dark:border-blue-800 dark:bg-blue-900 dark:text-blue-100">
            <p class="font-medium text-blue-900 dark:text-blue-100">What happens next</p>
            <ul class="mt-2 list-disc space-y-1 pl-4 text-blue-800 dark:text-blue-200 text-sm">
              <li>Pulse removes the node entry and clears related alerts.</li>
              <li>
                {props.nodePendingDeleteHost ? (
                  <>
                    The host <span class="font-semibold">{props.nodePendingDeleteHost}</span>{' '}
                    loses the proxy service, SSH key, and API token.
                  </>
                ) : (
                  'The host loses the proxy service, SSH key, and API token.'
                )}
              </li>
              <li>
                If the host comes back later, rerunning the setup script reinstalls everything with
                a fresh key.
              </li>
              <Show when={props.nodePendingDeleteType === 'pbs'}>
                <li>
                  Backup user tokens on the PBS are removed, so jobs referencing them will no
                  longer authenticate until the node is re-added.
                </li>
              </Show>
              <Show when={props.nodePendingDeleteType === 'pmg'}>
                <li>
                  Mail gateway tokens are removed as part of the cleanup; re-enroll to restore
                  outbound telemetry.
                </li>
              </Show>
            </ul>
          </div>
        </div>

        <div class="flex items-center justify-end gap-3 pt-2">
          <button
            type="button"
            onClick={props.onCancel}
            class="rounded-md border border-gray-300 px-4 py-2 text-sm font-medium text-base-content transition-colors hover:bg-surface-hover"
            disabled={props.deleteNodeLoading}
          >
            Keep node
          </button>
          <button
            type="button"
            onClick={props.onDelete}
            disabled={props.deleteNodeLoading}
            class="rounded-md bg-red-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-red-700 disabled:cursor-not-allowed disabled:opacity-60 dark:bg-red-500 dark:hover:bg-red-400"
          >
            {props.deleteNodeLoading ? 'Removing…' : 'Remove node'}
          </button>
        </div>
      </Card>
    </Dialog>
  );
};
