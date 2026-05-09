import { Component, For, Show } from 'solid-js';
import type { ProbeCandidate } from '@/api/connections';
import { formControl, formField, formHelpText, formLabel } from '@/components/shared/Form';
import { formatConnectionErrorMessage } from '@/utils/connectionErrorPresentation';
import { getInfrastructureAgentHostProfileSupportText } from '@/utils/infrastructureOnboardingPresentation';
import type { CompletedProbePhase, ConnectionEditorState } from './useConnectionEditor';
import { CONNECTION_TYPE_LABELS } from './useConnectionEditor';

export interface AddressProbeStepProps {
  state: ConnectionEditorState;
  onSelectCandidate: (candidate: ProbeCandidate) => void;
  onInstallAgent?: () => void;
  onChooseSourceTypeInstead?: () => void;
  onProbeSubmitted?: () => void;
  onProbeResolved?: (outcome: CompletedProbePhase) => void;
}

export const AddressProbeStep: Component<AddressProbeStepProps> = (props) => {
  const handleSubmit = async (event: SubmitEvent) => {
    event.preventDefault();
    if (props.state.address().trim().length === 0) {
      return;
    }

    props.onProbeSubmitted?.();
    const outcome = await props.state.runProbe();
    props.onProbeResolved?.(outcome);
  };

  return (
    <form class="space-y-4" onSubmit={handleSubmit}>
      <div class={formField}>
        <label class={formLabel} for="connection-address">
          API endpoint
        </label>
        <input
          id="connection-address"
          type="text"
          class={formControl}
          placeholder="vcenter.lab.local, truenas.lan, https://pve.lab:8006"
          value={props.state.address()}
          onInput={(event) => props.state.setAddress(event.currentTarget.value)}
          autocomplete="off"
          spellcheck={false}
          disabled={props.state.phase() === 'probing'}
        />
        <p class={formHelpText}>
          Paste a hostname, IP, or URL for a supported API-backed platform. Pulse validates the
          match and asks for credentials next.
        </p>
      </div>

      <div class="flex items-center gap-2">
        <button
          type="submit"
          class="inline-flex items-center rounded-md bg-blue-600 px-3 py-2 text-sm font-medium text-white transition-colors hover:bg-blue-500 disabled:opacity-60"
          disabled={props.state.phase() === 'probing' || props.state.address().trim().length === 0}
        >
          {props.state.phase() === 'probing' ? 'Probing…' : 'Probe API endpoint'}
        </button>
      </div>

      <Show when={props.state.phase() === 'error' && props.state.errorMessage().length > 0}>
        <div class="rounded-md border border-red-300 bg-red-50 px-3 py-2 text-sm text-red-800 dark:border-red-800 dark:bg-red-950/40 dark:text-red-200">
          {formatConnectionErrorMessage(props.state.errorMessage()) ?? props.state.errorMessage()}
        </div>
      </Show>

      <Show when={props.state.phase() === 'no-match'}>
        <div class="rounded-md border border-amber-300 bg-amber-50 px-3 py-2 text-sm text-amber-900 dark:border-amber-800 dark:bg-amber-950/40 dark:text-amber-100">
          <div class="font-medium">No supported API-backed platform detected at that address.</div>
          <div class="mt-1 text-xs">
            <Show when={props.onChooseSourceTypeInstead}>
              <button
                type="button"
                onClick={props.onChooseSourceTypeInstead}
                class="font-medium underline underline-offset-2 hover:text-amber-950 dark:hover:text-amber-50"
              >
                Choose a source type instead
              </button>
              <span>. </span>
            </Show>
            If this is one of the supported {getInfrastructureAgentHostProfileSupportText()},{' '}
            <Show
              when={props.onInstallAgent}
              fallback={<span class="font-medium">install Pulse Agent instead</span>}
            >
              <button
                type="button"
                onClick={props.onInstallAgent}
                class="font-medium underline underline-offset-2 hover:text-amber-950 dark:hover:text-amber-50"
              >
                install Pulse Agent instead
              </button>
            </Show>
            .
          </div>
        </div>
      </Show>

      <Show when={props.state.phase() === 'detected' && props.state.candidates().length > 0}>
        <div class="space-y-2">
          <div class="flex items-baseline justify-between">
            <div class="text-sm font-semibold text-base-content">Detected</div>
            <Show when={props.state.probedMs() > 0}>
              <div class="text-xs text-muted">Probed in {props.state.probedMs()} ms</div>
            </Show>
          </div>
          <ul class="divide-y divide-border rounded-md border border-border">
            <For each={props.state.candidates()}>
              {(candidate) => {
                const hintEntries = Object.entries(candidate.hints ?? {});
                return (
                  <li>
                    <button
                      type="button"
                      class="flex w-full flex-col items-start gap-1 px-3 py-2.5 text-left transition-colors hover:bg-surface-hover"
                      onClick={() => props.onSelectCandidate(candidate)}
                    >
                      <div class="text-sm font-medium text-base-content">
                        {CONNECTION_TYPE_LABELS[candidate.type] ?? candidate.type}
                      </div>
                      <div class="text-xs text-muted">{candidate.host}</div>
                      <Show when={hintEntries.length > 0}>
                        <div class="mt-0.5 flex flex-wrap gap-x-3 gap-y-0.5 text-[11px] text-muted">
                          <For each={hintEntries}>
                            {([key, value]) => (
                              <span>
                                <span class="font-medium">{key}:</span> {value}
                              </span>
                            )}
                          </For>
                        </div>
                      </Show>
                    </button>
                  </li>
                );
              }}
            </For>
          </ul>
        </div>
      </Show>
    </form>
  );
};
