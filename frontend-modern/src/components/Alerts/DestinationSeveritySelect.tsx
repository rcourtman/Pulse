import type { NotificationMinimumSeverity } from '@/api/notifications';
import { controlClass, formControl } from '@/components/shared/Form';
import { FormSelect } from '@/components/shared/FormSelect';
import {
  ALERT_DESTINATION_ALL_SEVERITIES_LABEL,
  ALERT_DESTINATION_CRITICAL_ONLY_LABEL,
  ALERT_DESTINATION_MINIMUM_SEVERITY_HELP,
  ALERT_DESTINATION_MINIMUM_SEVERITY_LABEL,
} from '@/utils/alertDestinationsPresentation';

interface DestinationSeveritySelectProps {
  id: string;
  value: NotificationMinimumSeverity;
  onChange: (value: NotificationMinimumSeverity) => void;
  compact?: boolean;
  help?: string;
}

export function DestinationSeveritySelect(props: DestinationSeveritySelectProps) {
  return (
    <FormSelect
      id={props.id}
      label={ALERT_DESTINATION_MINIMUM_SEVERITY_LABEL}
      labelClass={props.compact ? undefined : 'text-xs uppercase tracking-[0.08em]'}
      value={props.value}
      onChange={(event) =>
        props.onChange(event.currentTarget.value === 'critical' ? 'critical' : 'all')
      }
      selectBaseClass={props.compact ? controlClass('px-2 py-1.5') : formControl}
      help={props.help ?? ALERT_DESTINATION_MINIMUM_SEVERITY_HELP}
    >
      <option value="all">{ALERT_DESTINATION_ALL_SEVERITIES_LABEL}</option>
      <option value="critical">{ALERT_DESTINATION_CRITICAL_ONLY_LABEL}</option>
    </FormSelect>
  );
}
