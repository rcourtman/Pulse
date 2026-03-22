import { Show } from 'solid-js';

import { SettingsPanel } from '@/components/shared/SettingsPanel';
import { Toggle } from '@/components/shared/Toggle';
import {
  formControl,
  formField,
  formHelpText,
  labelClass,
} from '@/components/shared/Form';
import type { UIAppriseConfig } from './types';
import {
  ALERT_DESTINATIONS_APPRISE_API_KEY_HEADER_HELP,
  ALERT_DESTINATIONS_APPRISE_API_KEY_HEADER_LABEL,
  ALERT_DESTINATIONS_APPRISE_API_KEY_HEADER_PLACEHOLDER,
  ALERT_DESTINATIONS_APPRISE_API_KEY_HELP,
  ALERT_DESTINATIONS_APPRISE_API_KEY_LABEL,
  ALERT_DESTINATIONS_APPRISE_API_KEY_PLACEHOLDER,
  ALERT_DESTINATIONS_APPRISE_CLI_PATH_HELP,
  ALERT_DESTINATIONS_APPRISE_CLI_PATH_LABEL,
  ALERT_DESTINATIONS_APPRISE_CLI_PATH_PLACEHOLDER,
  ALERT_DESTINATIONS_APPRISE_CONFIG_KEY_HELP,
  ALERT_DESTINATIONS_APPRISE_CONFIG_KEY_LABEL,
  ALERT_DESTINATIONS_APPRISE_CONFIG_KEY_PLACEHOLDER,
  ALERT_DESTINATIONS_APPRISE_MODE_CLI_LABEL,
  ALERT_DESTINATIONS_APPRISE_MODE_HELP,
  ALERT_DESTINATIONS_APPRISE_MODE_HTTP_LABEL,
  ALERT_DESTINATIONS_APPRISE_MODE_LABEL,
  ALERT_DESTINATIONS_APPRISE_PANEL_DESCRIPTION,
  ALERT_DESTINATIONS_APPRISE_PANEL_TITLE,
  ALERT_DESTINATIONS_APPRISE_SERVER_URL_HELP,
  ALERT_DESTINATIONS_APPRISE_SERVER_URL_LABEL,
  ALERT_DESTINATIONS_APPRISE_SERVER_URL_PLACEHOLDER,
  ALERT_DESTINATIONS_APPRISE_TARGETS_LABEL,
  ALERT_DESTINATIONS_APPRISE_TARGETS_PLACEHOLDER,
  ALERT_DESTINATIONS_APPRISE_TIMEOUT_HELP,
  ALERT_DESTINATIONS_APPRISE_TIMEOUT_LABEL,
  ALERT_DESTINATIONS_APPRISE_TLS_CHECKBOX_LABEL,
  ALERT_DESTINATIONS_APPRISE_TLS_HELP,
  ALERT_DESTINATIONS_APPRISE_TLS_LABEL,
  getAlertDestinationsAppriseTargetsHelp,
  getAlertDestinationsAppriseTestLabel,
  getAlertDestinationsStatusLabel,
} from '@/utils/alertDestinationsPresentation';

interface AlertAppriseDestinationsSectionProps {
  config: UIAppriseConfig;
  updateApprise: (partial: Partial<UIAppriseConfig>) => void;
  setHasUnsavedChanges: (value: boolean) => void;
  onTest: () => void;
  testing: boolean;
}

export function AlertAppriseDestinationsSection(props: AlertAppriseDestinationsSectionProps) {
  return (
    <SettingsPanel
      title={ALERT_DESTINATIONS_APPRISE_PANEL_TITLE}
      description={ALERT_DESTINATIONS_APPRISE_PANEL_DESCRIPTION}
      action={
        <div class="flex items-center gap-3 sm:self-start">
          <Toggle
            checked={props.config.enabled}
            onChange={(event) => {
              props.updateApprise({ enabled: event.currentTarget.checked });
              props.setHasUnsavedChanges(true);
            }}
            label={
              <span class="text-xs font-medium text-muted">
                {getAlertDestinationsStatusLabel(props.config.enabled)}
              </span>
            }
          />
          <button
            class="rounded-md border border-border px-3 py-2 text-sm font-medium text-base-content transition hover:bg-surface-hover disabled:cursor-not-allowed disabled:opacity-60"
            disabled={!props.config.enabled || props.testing}
            onClick={props.onTest}
          >
            {getAlertDestinationsAppriseTestLabel(props.testing)}
          </button>
        </div>
      }
      class="min-w-0"
      bodyClass="space-y-4"
    >
      <div class="space-y-4">
        <div class={formField}>
          <label class={labelClass('text-xs uppercase tracking-[0.08em]')}>
            {ALERT_DESTINATIONS_APPRISE_MODE_LABEL}
          </label>
          <select
            class={formControl}
            value={props.config.mode}
            onInput={(event) => {
              props.updateApprise({ mode: event.currentTarget.value as 'cli' | 'http' });
              props.setHasUnsavedChanges(true);
            }}
          >
            <option value="cli">{ALERT_DESTINATIONS_APPRISE_MODE_CLI_LABEL}</option>
            <option value="http">{ALERT_DESTINATIONS_APPRISE_MODE_HTTP_LABEL}</option>
          </select>
          <p class={formHelpText}>{ALERT_DESTINATIONS_APPRISE_MODE_HELP}</p>
        </div>

        <div class={formField}>
          <label class={labelClass('text-xs uppercase tracking-[0.08em]')}>
            {ALERT_DESTINATIONS_APPRISE_TARGETS_LABEL}
          </label>
          <textarea
            rows={4}
            class={`${formControl} min-h-[120px] font-mono`}
            value={props.config.targetsText}
            placeholder={ALERT_DESTINATIONS_APPRISE_TARGETS_PLACEHOLDER}
            onInput={(event) => {
              props.updateApprise({ targetsText: event.currentTarget.value });
              props.setHasUnsavedChanges(true);
            }}
          />
          <p class={formHelpText}>{getAlertDestinationsAppriseTargetsHelp(props.config.mode)}</p>
        </div>

        <Show when={props.config.mode === 'cli'}>
          <div class={formField}>
            <label class={labelClass('text-xs uppercase tracking-[0.08em]')}>
              {ALERT_DESTINATIONS_APPRISE_CLI_PATH_LABEL}
            </label>
            <input
              type="text"
              value={props.config.cliPath}
              class={formControl}
              placeholder={ALERT_DESTINATIONS_APPRISE_CLI_PATH_PLACEHOLDER}
              onInput={(event) => {
                props.updateApprise({ cliPath: event.currentTarget.value });
                props.setHasUnsavedChanges(true);
              }}
            />
            <p class={formHelpText}>{ALERT_DESTINATIONS_APPRISE_CLI_PATH_HELP}</p>
          </div>
        </Show>

        <Show when={props.config.mode === 'http'}>
          <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <div class={`${formField} sm:col-span-2`}>
              <label class={labelClass('text-xs uppercase tracking-[0.08em]')}>
                {ALERT_DESTINATIONS_APPRISE_SERVER_URL_LABEL}
              </label>
              <input
                type="text"
                value={props.config.serverUrl}
                class={formControl}
                placeholder={ALERT_DESTINATIONS_APPRISE_SERVER_URL_PLACEHOLDER}
                onInput={(event) => {
                  props.updateApprise({ serverUrl: event.currentTarget.value });
                  props.setHasUnsavedChanges(true);
                }}
              />
              <p class={formHelpText}>{ALERT_DESTINATIONS_APPRISE_SERVER_URL_HELP}</p>
            </div>
            <div class={formField}>
              <label class={labelClass('text-xs uppercase tracking-[0.08em]')}>
                {ALERT_DESTINATIONS_APPRISE_CONFIG_KEY_LABEL}
              </label>
              <input
                type="text"
                value={props.config.configKey}
                class={formControl}
                placeholder={ALERT_DESTINATIONS_APPRISE_CONFIG_KEY_PLACEHOLDER}
                onInput={(event) => {
                  props.updateApprise({ configKey: event.currentTarget.value });
                  props.setHasUnsavedChanges(true);
                }}
              />
              <p class={formHelpText}>{ALERT_DESTINATIONS_APPRISE_CONFIG_KEY_HELP}</p>
            </div>
            <div class={formField}>
              <label class={labelClass('text-xs uppercase tracking-[0.08em]')}>
                {ALERT_DESTINATIONS_APPRISE_API_KEY_LABEL}
              </label>
              <input
                type="password"
                value={props.config.apiKey}
                class={formControl}
                placeholder={ALERT_DESTINATIONS_APPRISE_API_KEY_PLACEHOLDER}
                onInput={(event) => {
                  props.updateApprise({ apiKey: event.currentTarget.value });
                  props.setHasUnsavedChanges(true);
                }}
              />
              <p class={formHelpText}>{ALERT_DESTINATIONS_APPRISE_API_KEY_HELP}</p>
            </div>
            <div class={formField}>
              <label class={labelClass('text-xs uppercase tracking-[0.08em]')}>
                {ALERT_DESTINATIONS_APPRISE_API_KEY_HEADER_LABEL}
              </label>
              <input
                type="text"
                value={props.config.apiKeyHeader}
                class={formControl}
                placeholder={ALERT_DESTINATIONS_APPRISE_API_KEY_HEADER_PLACEHOLDER}
                onInput={(event) => {
                  props.updateApprise({ apiKeyHeader: event.currentTarget.value });
                  props.setHasUnsavedChanges(true);
                }}
              />
              <p class={formHelpText}>{ALERT_DESTINATIONS_APPRISE_API_KEY_HEADER_HELP}</p>
            </div>
            <div class={`${formField} sm:col-span-2`}>
              <label class={labelClass('text-xs uppercase tracking-[0.08em]')}>
                {ALERT_DESTINATIONS_APPRISE_TLS_LABEL}
              </label>
              <label class="inline-flex items-center gap-2">
                <input
                  type="checkbox"
                  class="h-4 w-4 rounded border border-border"
                  checked={props.config.skipTlsVerify}
                  onChange={(event) => {
                    props.updateApprise({ skipTlsVerify: event.currentTarget.checked });
                    props.setHasUnsavedChanges(true);
                  }}
                />
                <span class="text-sm text-muted">
                  {ALERT_DESTINATIONS_APPRISE_TLS_CHECKBOX_LABEL}
                </span>
              </label>
              <p class={formHelpText}>{ALERT_DESTINATIONS_APPRISE_TLS_HELP}</p>
            </div>
          </div>
        </Show>

        <div class={formField}>
          <label class={labelClass('text-xs uppercase tracking-[0.08em]')}>
            {ALERT_DESTINATIONS_APPRISE_TIMEOUT_LABEL}
          </label>
          <input
            type="number"
            min="5"
            max="120"
            value={props.config.timeoutSeconds}
            class={formControl}
            onInput={(event) => {
              const raw = event.currentTarget.valueAsNumber;
              const safe = Number.isNaN(raw) ? 15 : Math.min(120, Math.max(5, Math.trunc(raw)));
              props.updateApprise({ timeoutSeconds: safe });
              props.setHasUnsavedChanges(true);
            }}
          />
          <p class={formHelpText}>{ALERT_DESTINATIONS_APPRISE_TIMEOUT_HELP}</p>
        </div>
      </div>
    </SettingsPanel>
  );
}
