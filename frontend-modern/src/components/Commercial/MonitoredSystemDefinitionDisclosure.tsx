import { Show, createSignal, type Component } from 'solid-js';
import {
  getMonitoredSystemBriefSummary,
  getMonitoredSystemDisclosureDefinition,
  getMonitoredSystemDisclosureToggleLabel,
} from '@/utils/monitoredSystemPresentation';

interface MonitoredSystemDefinitionDisclosureProps {
  showSummary?: boolean;
  class?: string;
  summaryClass?: string;
  buttonClass?: string;
  detailClass?: string;
}

export const MonitoredSystemDefinitionDisclosure: Component<
  MonitoredSystemDefinitionDisclosureProps
> = (props) => {
  const [open, setOpen] = createSignal(false);

  return (
    <div class={props.class ?? 'space-y-2'}>
      <Show when={props.showSummary}>
        <p class={props.summaryClass ?? 'text-xs text-muted'}>
          {getMonitoredSystemBriefSummary()}
        </p>
      </Show>

      <div class="space-y-2">
        <button
          type="button"
          class={
            props.buttonClass ??
            'text-xs font-medium text-muted underline-offset-2 transition-colors hover:text-base-content hover:underline'
          }
          aria-expanded={open()}
          onClick={() => setOpen((current) => !current)}
        >
          {getMonitoredSystemDisclosureToggleLabel(open())}
        </button>

        <Show when={open()}>
          <p class={props.detailClass ?? 'max-w-2xl text-xs text-muted'}>
            {getMonitoredSystemDisclosureDefinition()}
          </p>
        </Show>
      </div>
    </div>
  );
};

export default MonitoredSystemDefinitionDisclosure;
