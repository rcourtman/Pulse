import { createUniqueId } from 'solid-js';

import { formHelpText } from '@/components/shared/Form';
import { FormSelect } from '@/components/shared/FormSelect';
import { SettingsPanel } from '@/components/shared/SettingsPanel';
import {
  ALERT_CONFIG_DELIVERY_DESCRIPTION,
  ALERT_CONFIG_DELIVERY_HELP,
  ALERT_CONFIG_DELIVERY_TARGET_LABEL,
  ALERT_CONFIG_DELIVERY_TITLE,
  getAlertConfigEscalationNotifyLabel,
} from '@/utils/alertConfigPresentation';

import type { NotificationDeliveryTarget } from './types';

interface AlertDeliveryRoutingSectionProps {
  initialNotify: NotificationDeliveryTarget;
  setInitialNotifyTarget: (value: NotificationDeliveryTarget) => void;
}

export function AlertDeliveryRoutingSection(props: AlertDeliveryRoutingSectionProps) {
  const fieldId = `alert-initial-delivery-${createUniqueId()}`;

  return (
    <SettingsPanel
      title={ALERT_CONFIG_DELIVERY_TITLE}
      description={ALERT_CONFIG_DELIVERY_DESCRIPTION}
    >
      <div class="space-y-3">
        <FormSelect
          id={fieldId}
          label={ALERT_CONFIG_DELIVERY_TARGET_LABEL}
          value={props.initialNotify}
          onChange={(event) =>
            props.setInitialNotifyTarget(event.currentTarget.value as NotificationDeliveryTarget)
          }
        >
          <option value="all">{getAlertConfigEscalationNotifyLabel('all')}</option>
          <option value="email">{getAlertConfigEscalationNotifyLabel('email')}</option>
          <option value="webhook">{getAlertConfigEscalationNotifyLabel('webhook')}</option>
          <option value="apprise">{getAlertConfigEscalationNotifyLabel('apprise')}</option>
        </FormSelect>
        <p class={formHelpText}>{ALERT_CONFIG_DELIVERY_HELP}</p>
      </div>
    </SettingsPanel>
  );
}
