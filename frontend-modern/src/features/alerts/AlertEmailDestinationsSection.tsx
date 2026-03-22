import { EmailProviderSelect } from '@/components/Alerts/EmailProviderSelect';
import { SettingsPanel } from '@/components/shared/SettingsPanel';
import { Toggle } from '@/components/shared/Toggle';
import type { UIEmailConfig } from './types';
import {
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
  return (
    <SettingsPanel
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
      <div
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
    </SettingsPanel>
  );
}
