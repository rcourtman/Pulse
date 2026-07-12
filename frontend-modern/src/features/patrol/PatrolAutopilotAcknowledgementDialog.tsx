import { For, createSignal, type Component } from 'solid-js';
import XIcon from 'lucide-solid/icons/x';
import { Button } from '@/components/shared/Button';
import { Dialog } from '@/components/shared/Dialog';
import type { PatrolIntelligenceState } from './usePatrolIntelligenceState';

const scopeLabel = (scope: string): string => {
  switch (scope) {
    case 'policy_authorized_actions': return 'Only actions authorized by current server policy';
    case 'capability_allowlisted_only': return 'Only capabilities on the server allowlist';
    case 'outcome_truth_not_inferred': return 'Execution success is never presented as verified outcome truth';
    case 'revocation_and_version_rotation_bound': return 'Activation stops after revocation or acknowledgement version rotation';
    default: return scope.replace(/_/g, ' ');
  }
};

export const PatrolAutopilotAcknowledgementDialog: Component<{ state: PatrolIntelligenceState }> = (props) => {
  const [accepted, setAccepted] = createSignal(false);
  const status = () => props.state.autopilotStatus();
  const close = () => { setAccepted(false); props.state.setAutopilotDialogOpen(false); };

  return (
    <Dialog isOpen={props.state.autopilotDialogOpen()} onClose={close} ariaLabelledBy="autopilot-ack-title" ariaDescribedBy="autopilot-ack-description" closeOnBackdrop={!props.state.isUpdatingAutonomy()} panelClass="max-w-2xl">
      <div class="flex max-h-[min(90vh,800px)] flex-col">
        <header class="flex items-start justify-between gap-4 border-b border-border px-5 py-4">
          <div><p class="text-xs font-semibold uppercase tracking-wide text-amber-700 dark:text-amber-300">Explicit control change</p><h2 id="autopilot-ack-title" class="mt-1 text-xl font-semibold">Activate Autopilot</h2><p id="autopilot-ack-description" class="mt-2 text-sm text-muted">Pulse will record a server-owned version {status()?.currentVersion ?? 'current'} acknowledgement before requesting full mode.</p></div>
          <Button variant="ghost" size="icon" aria-label="Close Autopilot acknowledgement" onClick={close}><XIcon class="h-5 w-5" /></Button>
        </header>
        <div class="overflow-y-auto px-5 py-4">
          <div class="rounded-lg border border-amber-300 bg-amber-50 p-4 text-sm text-amber-950 dark:border-amber-800 dark:bg-amber-950/30 dark:text-amber-100">
            <p class="font-semibold">Autopilot may execute eligible infrastructure actions without asking each time.</p>
            <p class="mt-2">It remains bounded by approval floors, per-resource policy, the emergency stop, current licensing, and fresh execution-time authorization.</p>
          </div>
          <h3 class="mt-5 text-sm font-semibold">What this acknowledgement covers</h3>
          <ul class="mt-2 space-y-2 text-sm"><For each={status()?.acceptedScope ?? ['policy_authorized_actions', 'capability_allowlisted_only', 'outcome_truth_not_inferred']}>{(scope) => <li class="flex gap-2"><span aria-hidden="true">•</span><span>{scopeLabel(scope)}</span></li>}</For></ul>
          <h3 class="mt-5 text-sm font-semibold">Truth and recovery limits</h3>
          <ul class="mt-2 list-disc space-y-2 pl-5 text-sm text-muted">
            <li>Verification can be confirmed, contradicted, inconclusive, or not attempted.</li>
            <li>Pulse discloses whether evidence came from the executing agent or an independent observer.</li>
            <li>You can revoke this acknowledgement. Version changes, expiry, revocation, policy drift, or reconnect races demote effective mode server-side.</li>
          </ul>
          <label class="mt-5 flex cursor-pointer items-start gap-3 rounded-lg border border-border bg-surface-hover p-4 text-sm"><input type="checkbox" class="mt-1 h-4 w-4" checked={accepted()} onChange={(event) => setAccepted(event.currentTarget.checked)} /><span><strong>I understand and accept these Autopilot limits.</strong><span class="mt-1 block text-muted">This acknowledgement applies to my authenticated organization and identity.</span></span></label>
        </div>
        <footer class="flex flex-col-reverse gap-2 border-t border-border px-5 py-4 sm:flex-row sm:justify-end"><Button onClick={close}>Cancel</Button><Button variant="warningSolid" disabled={!accepted()} isLoading={props.state.isUpdatingAutonomy()} onClick={() => void props.state.acknowledgeAndActivateAutopilot()}>Record acknowledgement and activate</Button></footer>
      </div>
    </Dialog>
  );
};
