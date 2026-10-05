import { Show, createSignal, createUniqueId } from 'solid-js';

import { EmailProviderSelect } from '@/components/Alerts/EmailProviderSelect';
import { SettingsPanel } from '@/components/shared/SettingsPanel';
import { Toggle } from '@/components/shared/Toggle';
import type { UIEmailConfig } from './types';
import { AlertDestinationOffSummary } from './AlertDestinationOffSummary';
import {
  ALERT_DESTINATIONS_EMAIL_OFF_MESSAGE,
  ALERT_DESTINATIONS_EMAIL_PANEL_DESCRIPTION,
  ALERT_DESTINATIONS_EMAIL_PANEL_TITLE,
  getAlertDestinationsStatusLabel,
} from '@/utils/alertDestinationsPresentation';

interface AlertEmailDestinationsSectionProps {
  config: UIEmailConfig;
  setConfig: (config: UIEmailConfig) => void;
  setHasUnsavedChanges: (value: boolean) => void;
  onTest: () => void;
  testing: boolean;
}

export function AlertEmailDestinationsSection(props: AlertEmailDestinationsSectionProps) {
  const titleId = `alert-email-destinations-${createUniqueId()}-title`;
  const [showSettings, setShowSettings] = createSignal(false);
  let settingsRegion: HTMLDivElement | undefined;
  // The summary's button unmounts as the form renders, so hand keyboard focus
  // to the first revealed control instead of dropping it on the document.
  const revealSettings = () => {
    setShowSettings(true);
    settingsRegion?.querySelector<HTMLElement>('input, select, textarea, button')?.focus();
  };

  return (
    <SettingsPanel
      titleId={titleId}
      title={ALERT_DESTINATIONS_EMAIL_PANEL_TITLE}
      description={ALERT_DESTINATIONS_EMAIL_PANEL_DESCRIPTION}
      action={
        <Toggle
          checked={props.config.enabled}
          onChange={(event) => {
            props.setConfig({
              ...props.config,
              enabled: event.currentTarget.checked,
            });
            props.setHasUnsavedChanges(true);
          }}
          containerClass="sm:self-start"
          ariaLabelledBy={titleId}
          label={
            <span class="text-xs font-medium text-muted">
              {getAlertDestinationsStatusLabel(props.config.enabled)}
            </span>
          }
        />
      }
      class="min-w-0"
      bodyClass=""
    >
      <Show
        when={props.config.enabled || showSettings()}
        fallback={
          <AlertDestinationOffSummary
            message={ALERT_DESTINATIONS_EMAIL_OFF_MESSAGE}
            onShowSettings={revealSettings}
          />
        }
      >
        <div
          ref={settingsRegion}
          class={`${!props.config.enabled ? 'pointer-events-none opacity-50 transition-opacity' : 'transition-opacity'}`}
        >
          <EmailProviderSelect
            config={props.config}
            onChange={(config) => {
              props.setConfig(config);
              props.setHasUnsavedChanges(true);
            }}
            onTest={props.onTest}
            testing={props.testing}
          />
        </div>
      </Show>
    </SettingsPanel>
  );
}
