import { SettingsPanel } from '@/components/shared/SettingsPanel';
import { Toggle } from '@/components/shared/Toggle';
import { formHelpText } from '@/components/shared/Form';
import {
  ALERT_CONFIG_RECOVERY_DESCRIPTION,
  ALERT_CONFIG_RECOVERY_TITLE,
  getAlertConfigRecoveryHelp,
  getAlertConfigToggleStatusLabel,
} from '@/utils/alertConfigPresentation';

interface AlertRecoverySectionProps {
  notifyOnResolve: boolean;
  setNotifyOnResolveEnabled: (value: boolean) => void;
}

export function AlertRecoverySection(props: AlertRecoverySectionProps) {
  return (
    <SettingsPanel
      title={ALERT_CONFIG_RECOVERY_TITLE}
      description={ALERT_CONFIG_RECOVERY_DESCRIPTION}
      action={
        <Toggle
          checked={props.notifyOnResolve}
          onChange={(event) => props.setNotifyOnResolveEnabled(event.currentTarget.checked)}
          containerClass="sm:self-start"
          label={
            <span class="text-xs font-medium text-muted">
              {getAlertConfigToggleStatusLabel(props.notifyOnResolve)}
            </span>
          }
        />
      }
      class="space-y-3"
    >
      <p class={formHelpText}>{getAlertConfigRecoveryHelp()}</p>
    </SettingsPanel>
  );
}
