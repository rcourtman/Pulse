import { Component, Show, Switch, Match, createMemo } from 'solid-js';
import { Dialog } from '@/components/shared/Dialog';
import { StepIndicator } from '@/components/SetupWizard/StepIndicator';
import { useDeployWizard, type WizardStep } from '@/hooks/useDeployWizard';
import { CandidatesStep } from './deploy/CandidatesStep';
import { PreflightStep } from './deploy/PreflightStep';
import { ConfirmStep } from './deploy/ConfirmStep';
import { DeployingStep } from './deploy/DeployingStep';
import { ResultsStep } from './deploy/ResultsStep';
import XIcon from 'lucide-solid/icons/x';

interface AgentDeployModalProps {
  isOpen: boolean;
  clusterId: string;
  clusterName: string;
  onClose: () => void;
}

const STEP_LABELS = ['Select Hosts', 'Preflight', 'Confirm', 'Deploy', 'Results'];

const stepToIndex: Record<WizardStep, number> = {
  candidates: 0,
  preflight: 1,
  confirm: 2,
  deploying: 3,
  results: 4,
};

export const AgentDeployModal: Component<AgentDeployModalProps> = (props) => {
  const wizard = useDeployWizard({
    clusterId: props.clusterId,
    clusterName: props.clusterName,
  });

  const currentStepIndex = createMemo(() => stepToIndex[wizard.step()]);

  const canRunPreflight = createMemo(
    () =>
      wizard.selectedNodeIds().size > 0 &&
      wizard.selectedSourceAgent() !== '' &&
      !wizard.startingPreflight(),
  );

  const handleClose = () => {
    if (wizard.isOperationActive()) {
      // Don't block close — just warn (closing won't cancel).
      // The deployment continues in the background.
    }
    props.onClose();
  };

  return (
    <Dialog
      isOpen={props.isOpen}
      onClose={handleClose}
      panelClass="max-w-4xl"
      closeOnBackdrop={!wizard.isOperationActive()}
      ariaLabel={`Deploy Agents — ${props.clusterName}`}
    >
      {/* Header */}
      <div class="flex items-center justify-between border-b border-border px-4 py-3">
        <h2 class="text-sm font-semibold text-base-content">Deploy Agents — {props.clusterName}</h2>
        <button
          type="button"
          onClick={handleClose}
          class="rounded-md p-1 text-muted hover:text-base-content hover:bg-surface-hover transition-colors"
          aria-label="Close"
        >
          <XIcon class="w-4 h-4" />
        </button>
      </div>

      {/* Step indicator */}
      <nav aria-label="Deploy progress" class="border-b border-border px-4 py-2">
        <StepIndicator steps={STEP_LABELS} currentStep={currentStepIndex()} />
      </nav>

      {/* Body */}
      <div class="overflow-y-auto p-4 max-h-[60vh]" aria-live="polite">
        <Switch>
          <Match when={wizard.step() === 'candidates'}>
            <CandidatesStep wizard={wizard} />
          </Match>
          <Match when={wizard.step() === 'preflight'}>
            <PreflightStep wizard={wizard} />
          </Match>
          <Match when={wizard.step() === 'confirm'}>
            <ConfirmStep wizard={wizard} />
          </Match>
          <Match when={wizard.step() === 'deploying'}>
            <DeployingStep wizard={wizard} />
          </Match>
          <Match when={wizard.step() === 'results'}>
            <ResultsStep wizard={wizard} />
          </Match>
        </Switch>
      </div>

      {/* Footer */}
      <div class="flex items-center justify-between border-t border-border px-4 py-3">
        <div class="flex gap-2">
          <Switch>
            <Match when={wizard.step() === 'candidates'}>
              <button
                type="button"
                onClick={props.onClose}
                class="rounded-md border border-border px-3 py-1.5 text-xs font-medium text-muted hover:bg-surface-hover transition-colors"
              >
                Cancel
              </button>
            </Match>
            <Match when={wizard.step() === 'confirm'}>
              <button
                type="button"
                onClick={() => wizard.setStep('candidates')}
                class="rounded-md border border-border px-3 py-1.5 text-xs font-medium text-muted hover:bg-surface-hover transition-colors"
              >
                Back
              </button>
            </Match>
            <Match when={wizard.step() === 'preflight'}>
              <span class="text-xs text-muted">Preflight in progress...</span>
            </Match>
            <Match when={wizard.step() === 'deploying'}>
              <span class="text-xs text-muted">
                <Show when={wizard.isOperationActive()}>Closing won't cancel the deployment.</Show>
              </span>
            </Match>
          </Switch>
        </div>

        <div class="flex gap-2">
          <Switch>
            <Match when={wizard.step() === 'candidates'}>
              <button
                type="button"
                onClick={() => wizard.startPreflight()}
                disabled={!canRunPreflight()}
                class="rounded-md bg-blue-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
              >
                <Show when={wizard.startingPreflight()} fallback="Run Preflight">
                  <span class="flex items-center gap-1.5">
                    <span class="h-3 w-3 animate-spin rounded-full border-2 border-white border-t-transparent" />
                    Starting...
                  </span>
                </Show>
              </button>
            </Match>
            <Match when={wizard.step() === 'confirm'}>
              <button
                type="button"
                onClick={() => wizard.startDeploy()}
                disabled={wizard.confirmSelectedNodeIds().size === 0 || wizard.startingDeploy()}
                class="rounded-md bg-blue-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
              >
                <Show
                  when={wizard.startingDeploy()}
                  fallback={`Deploy ${wizard.confirmSelectedNodeIds().size} Host${wizard.confirmSelectedNodeIds().size !== 1 ? 's' : ''}`}
                >
                  <span class="flex items-center gap-1.5">
                    <span class="h-3 w-3 animate-spin rounded-full border-2 border-white border-t-transparent" />
                    Starting...
                  </span>
                </Show>
              </button>
            </Match>
            <Match when={wizard.step() === 'deploying'}>
              <button
                type="button"
                onClick={() => wizard.cancelDeploy()}
                disabled={wizard.canceling()}
                class="rounded-md bg-red-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-red-700 disabled:opacity-50 transition-colors"
              >
                <Show when={wizard.canceling()} fallback="Cancel Deployment">
                  Canceling...
                </Show>
              </button>
            </Match>
            <Match when={wizard.step() === 'results'}>
              <Show when={wizard.retryableTargets().length > 0}>
                <button
                  type="button"
                  onClick={() => wizard.retryFailed()}
                  disabled={wizard.retrying()}
                  class="rounded-md bg-amber-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-amber-700 disabled:opacity-50 transition-colors"
                >
                  <Show
                    when={wizard.retrying()}
                    fallback={`Retry ${wizard.retryableTargets().length} Failed`}
                  >
                    Retrying...
                  </Show>
                </button>
              </Show>
              <button
                type="button"
                onClick={props.onClose}
                class="rounded-md border border-border px-3 py-1.5 text-xs font-medium text-base-content hover:bg-surface-hover transition-colors"
              >
                Close
              </button>
            </Match>
          </Switch>
        </div>
      </div>
    </Dialog>
  );
};
