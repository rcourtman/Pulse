import { Show, createSignal } from 'solid-js';
import { Activity, Check, Loader2 } from 'lucide-solid';

import {
  AvailabilityTargetsAPI,
  type AvailabilityProbeProtocol,
  type AvailabilityTarget,
} from '@/api/availabilityTargets';
import type { AvailabilityProbeSuggestion } from '@/types/discovery';
import { InfoCardFrame } from '@/components/shared/InfoCardFrame';

interface AvailabilityProbeSuggestionCardProps {
  suggestion: AvailabilityProbeSuggestion;
  linkedResourceId: string;
}

type State = 'idle' | 'creating' | 'created' | 'error';

export function AvailabilityProbeSuggestionCard(props: AvailabilityProbeSuggestionCardProps) {
  const [state, setState] = createSignal<State>('idle');
  const [error, setError] = createSignal<string>('');

  const protocolLabel = () => {
    const proto = props.suggestion.protocol.toUpperCase();
    const port = props.suggestion.port ? ` :${props.suggestion.port}` : '';
    return `${proto}${port}`;
  };

  const handleCreate = async () => {
    setState('creating');
    setError('');
    const s = props.suggestion;
    const target: AvailabilityTarget = {
      id: '',
      name: s.service_name || `${s.protocol} probe`,
      address: s.address,
      protocol: s.protocol as AvailabilityProbeProtocol,
      port: s.port || undefined,
      path: s.protocol === 'http' || s.protocol === 'https' ? s.path || undefined : undefined,
      linkedResourceId: props.linkedResourceId,
      enabled: true,
    };
    try {
      await AvailabilityTargetsAPI.create(target);
      setState('created');
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to create probe');
      setState('error');
    }
  };

  return (
    <InfoCardFrame data-testid="availability-probe-suggestion">
      <div class="flex items-center justify-between gap-2 mb-2">
        <div class="flex min-w-0 items-center gap-1.5">
          <Activity class="h-3.5 w-3.5 text-emerald-600 dark:text-emerald-400" aria-hidden="true" />
          <h3 class="truncate text-[11px] font-medium uppercase tracking-wide text-base-content">
            Availability Monitoring
          </h3>
        </div>
        <span class="shrink-0 rounded bg-surface-alt px-1.5 py-0.5 text-[10px] font-medium text-muted">
          Suggested
        </span>
      </div>
      <div class="space-y-1.5 text-[11px]">
        <Show when={state() !== 'created'}>
          <div class="flex items-center justify-between gap-2">
            <span class="text-muted">Service</span>
            <span
              class="font-medium text-base-content truncate ml-2"
              title={props.suggestion.service_name}
            >
              {props.suggestion.service_name}
            </span>
          </div>
          <div class="flex items-center justify-between gap-2">
            <span class="text-muted">Probe</span>
            <span class="font-medium text-base-content">{protocolLabel()}</span>
          </div>
          <div class="flex items-center justify-between gap-2">
            <span class="text-muted">Target</span>
            <span
              class="font-medium text-base-content truncate ml-2"
              title={props.suggestion.address}
            >
              {props.suggestion.address}
            </span>
          </div>
          <Show when={state() === 'error'}>
            <div class="text-[10px] text-red-600 dark:text-red-400 mt-1">{error()}</div>
          </Show>
          <button
            type="button"
            disabled={state() === 'creating'}
            onClick={handleCreate}
            class="mt-2 w-full inline-flex items-center justify-center gap-1.5 rounded-md bg-emerald-600 px-3 py-1.5 text-[11px] font-semibold text-white hover:bg-emerald-700 disabled:opacity-50 focus:outline-none focus-visible:ring-2 focus-visible:ring-emerald-500"
          >
            <Show when={state() === 'creating'}>
              <Loader2 class="h-3 w-3 animate-spin" aria-hidden="true" />
            </Show>
            {state() === 'creating' ? 'Creating…' : 'Monitor availability'}
          </button>
        </Show>
        <Show when={state() === 'created'}>
          <div class="flex items-center gap-1.5 text-emerald-600 dark:text-emerald-400">
            <Check class="h-3.5 w-3.5" aria-hidden="true" />
            <span class="font-medium">Probe created. Status will appear shortly.</span>
          </div>
        </Show>
      </div>
    </InfoCardFrame>
  );
}
