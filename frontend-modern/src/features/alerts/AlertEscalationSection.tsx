import { For, Show, createUniqueId } from 'solid-js';

import { controlClass, formHelpText } from '@/components/shared/Form';
import { SettingsPanel } from '@/components/shared/SettingsPanel';
import { Toggle } from '@/components/shared/Toggle';
import {
  ALERT_CONFIG_ESCALATION_ADD_LABEL,
  ALERT_CONFIG_ESCALATION_AFTER_LABEL,
  ALERT_CONFIG_ESCALATION_DESCRIPTION,
  ALERT_CONFIG_ESCALATION_DESTINATIONS_HELP,
  ALERT_CONFIG_ESCALATION_DESTINATION_DISABLED,
  ALERT_CONFIG_ESCALATION_DESTINATION_UNAVAILABLE,
  ALERT_CONFIG_ESCALATION_MINUTES_SUFFIX,
  ALERT_CONFIG_ESCALATION_NOTIFY_LABEL,
  ALERT_CONFIG_ESCALATION_REMOVE_TITLE,
  ALERT_CONFIG_ESCALATION_REPEAT_EVERY_LABEL,
  ALERT_CONFIG_ESCALATION_REPEAT_HELP,
  ALERT_CONFIG_ESCALATION_REPEAT_LABEL,
  ALERT_CONFIG_ESCALATION_TITLE,
  getAlertConfigEscalationHelp,
  getAlertConfigToggleStatusLabel,
} from '@/utils/alertConfigPresentation';

import type { EscalationConfig, EscalationDestination, EscalationNotifyTarget } from './types';

interface AlertEscalationSectionProps {
  escalation: EscalationConfig;
  setEscalationEnabled: (value: boolean) => void;
  setEscalationAfter: (index: number, value: string) => void;
  setEscalationNotify: (index: number, value: EscalationNotifyTarget) => void;
  destinations: EscalationDestination[];
  setEscalationDestinationIds: (index: number, destinationIds: string[]) => void;
  setEscalationRepeatCritical: (value: boolean) => void;
  setEscalationRepeatEvery: (value: string) => void;
  removeEscalationLevel: (index: number) => void;
  addEscalationLevel: () => void;
}

export function AlertEscalationSection(props: AlertEscalationSectionProps) {
  const fieldIdPrefix = `alert-escalation-${createUniqueId()}`;
  const titleId = `${fieldIdPrefix}-title`;

  return (
    <SettingsPanel
      titleId={titleId}
      title={ALERT_CONFIG_ESCALATION_TITLE}
      description={ALERT_CONFIG_ESCALATION_DESCRIPTION}
      action={
        <Toggle
          checked={props.escalation.enabled}
          onChange={(event) => props.setEscalationEnabled(event.currentTarget.checked)}
          containerClass="sm:self-start"
          ariaLabelledBy={titleId}
          label={
            <span class="text-xs font-medium text-muted">
              {getAlertConfigToggleStatusLabel(props.escalation.enabled)}
            </span>
          }
        />
      }
      class="space-y-4"
      noPadding={!props.escalation.enabled}
    >
      <Show when={props.escalation.enabled}>
        <div class="space-y-3">
          <p class={formHelpText}>{getAlertConfigEscalationHelp()}</p>
          <For each={props.escalation.levels}>
            {(level, index) => {
              const afterId = () => `${fieldIdPrefix}-after-${index()}`;
              const destinationsId = () => `${fieldIdPrefix}-destinations-${index()}`;
              const selectedDestinationIds = () =>
                level.destinationIds && level.destinationIds.length > 0
                  ? level.destinationIds
                  : props.destinations
                      .filter(
                        (destination) =>
                          level.notify === 'all' || destination.kind === level.notify,
                      )
                      .map((destination) => destination.id);
              const destinationOptions = () => [
                ...props.destinations,
                ...selectedDestinationIds()
                  .filter((id) => !props.destinations.some((destination) => destination.id === id))
                  .map(
                    (id) =>
                      ({
                        id,
                        label: `${ALERT_CONFIG_ESCALATION_DESTINATION_UNAVAILABLE} (${id})`,
                        kind: id === 'email' ? 'email' : id === 'apprise' ? 'apprise' : 'webhook',
                        enabled: false,
                      }) as EscalationDestination,
                  ),
              ];

              return (
                <div class="flex items-start gap-3 rounded-md border border-border bg-surface-hover p-3">
                  <div class="flex flex-1 flex-col gap-3">
                    <div class="flex items-center gap-2">
                      <label for={afterId()} class="text-xs font-medium text-muted">
                        {ALERT_CONFIG_ESCALATION_AFTER_LABEL}
                      </label>
                      <input
                        id={afterId()}
                        type="number"
                        min="5"
                        max="180"
                        value={level.after}
                        onChange={(event) =>
                          props.setEscalationAfter(index(), event.currentTarget.value)
                        }
                        class={`${controlClass('px-2 py-1 text-sm')} w-20`}
                      />
                      <span class="text-xs text-muted">
                        {ALERT_CONFIG_ESCALATION_MINUTES_SUFFIX}
                      </span>
                    </div>
                    <fieldset id={destinationsId()} class="space-y-2">
                      <legend class="text-xs font-medium text-muted">
                        {ALERT_CONFIG_ESCALATION_NOTIFY_LABEL}
                      </legend>
                      <div class="grid gap-2 sm:grid-cols-2">
                        <For each={destinationOptions()}>
                          {(destination) => {
                            const checked = () => selectedDestinationIds().includes(destination.id);
                            return (
                              <label class="flex min-w-0 items-center gap-2 rounded-md border border-border bg-surface px-2.5 py-2 text-sm">
                                <input
                                  type="checkbox"
                                  checked={checked()}
                                  disabled={checked() && selectedDestinationIds().length === 1}
                                  onChange={(event) => {
                                    const next = event.currentTarget.checked
                                      ? [...selectedDestinationIds(), destination.id]
                                      : selectedDestinationIds().filter(
                                          (id) => id !== destination.id,
                                        );
                                    props.setEscalationDestinationIds(index(), next);
                                  }}
                                  class="h-4 w-4 rounded border-border"
                                />
                                <span class="min-w-0 truncate">{destination.label}</span>
                                <Show when={!destination.enabled}>
                                  <span class="ml-auto text-xs text-muted">
                                    {ALERT_CONFIG_ESCALATION_DESTINATION_DISABLED}
                                  </span>
                                </Show>
                              </label>
                            );
                          }}
                        </For>
                      </div>
                      <p class={formHelpText}>{ALERT_CONFIG_ESCALATION_DESTINATIONS_HELP}</p>
                    </fieldset>
                  </div>
                  <button
                    type="button"
                    onClick={() => props.removeEscalationLevel(index())}
                    class="rounded-md p-1.5 text-red-600 transition-colors hover:bg-red-100 dark:hover:bg-red-900"
                    title={ALERT_CONFIG_ESCALATION_REMOVE_TITLE}
                    aria-label={ALERT_CONFIG_ESCALATION_REMOVE_TITLE}
                  >
                    <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        stroke-width="2"
                        d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
                      />
                    </svg>
                  </button>
                </div>
              );
            }}
          </For>

          <Show when={props.escalation.levels.length > 0}>
            <div class="space-y-3 rounded-md border border-border bg-surface p-3">
              <Toggle
                checked={props.escalation.repeatCritical}
                onChange={(event) => props.setEscalationRepeatCritical(event.currentTarget.checked)}
                label={
                  <span class="text-sm font-medium text-base-content">
                    {ALERT_CONFIG_ESCALATION_REPEAT_LABEL}
                  </span>
                }
              />
              <Show when={props.escalation.repeatCritical}>
                <div class="flex items-center gap-2">
                  <label
                    for={`${fieldIdPrefix}-repeat-every`}
                    class="text-xs font-medium text-muted"
                  >
                    {ALERT_CONFIG_ESCALATION_REPEAT_EVERY_LABEL}
                  </label>
                  <input
                    id={`${fieldIdPrefix}-repeat-every`}
                    type="number"
                    min="5"
                    max="180"
                    value={props.escalation.repeatEvery}
                    onChange={(event) => props.setEscalationRepeatEvery(event.currentTarget.value)}
                    class={`${controlClass('px-2 py-1 text-sm')} w-20`}
                  />
                  <span class="text-xs text-muted">{ALERT_CONFIG_ESCALATION_MINUTES_SUFFIX}</span>
                </div>
              </Show>
              <p class={formHelpText}>{ALERT_CONFIG_ESCALATION_REPEAT_HELP}</p>
            </div>
          </Show>

          <button
            type="button"
            onClick={props.addEscalationLevel}
            class="flex w-full items-center justify-center gap-2 rounded-md border-2 border-dashed border-border py-2 text-sm text-muted transition-all hover:border-slate-400 hover:bg-surface-hover"
          >
            <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M12 6v6m0 0v6m0-6h6m-6 0H6"
              />
            </svg>
            {ALERT_CONFIG_ESCALATION_ADD_LABEL}
          </button>
        </div>
      </Show>
    </SettingsPanel>
  );
}
