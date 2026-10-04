import { Show } from 'solid-js';
import { SettingsPanel } from '@/components/shared/SettingsPanel';
import {
  ALERT_DESTINATIONS_PUSH_MINIMUM_SEVERITY_HELP,
  ALERT_DESTINATIONS_PUSH_PANEL_DESCRIPTION,
  ALERT_DESTINATIONS_PUSH_PANEL_TITLE,
  ALERT_DESTINATIONS_PUSH_READY_MESSAGE,
  ALERT_DESTINATIONS_PUSH_SETUP_LINK_LABEL,
} from '@/utils/alertDestinationsPresentation';
import { DestinationSeveritySelect } from '@/components/Alerts/DestinationSeveritySelect';

interface AlertPushDestinationsSectionProps {
  relayLicensed: boolean;
  minimumSeverity?: 'all' | 'critical';
  onMinimumSeverityChange?: (value: 'all' | 'critical') => void;
}

// Pulse Mobile is being retired on 31 March 2027 and is no longer sold, so the
// panel only appears on instances that already have it; there is no upsell.
export function AlertPushDestinationsSection(props: AlertPushDestinationsSectionProps) {
  return (
    <Show when={props.relayLicensed}>
      <SettingsPanel
        title={ALERT_DESTINATIONS_PUSH_PANEL_TITLE}
        description={ALERT_DESTINATIONS_PUSH_PANEL_DESCRIPTION}
        class="min-w-0"
        bodyClass=""
      >
        <div class="flex flex-col gap-4">
          <p class="text-sm text-muted">{ALERT_DESTINATIONS_PUSH_READY_MESSAGE}</p>
          <DestinationSeveritySelect
            id="alert-push-minimum-severity"
            value={props.minimumSeverity ?? 'all'}
            includeWarning={false}
            onChange={(value) =>
              props.onMinimumSeverityChange?.(value === 'warning' ? 'all' : value)
            }
            help={ALERT_DESTINATIONS_PUSH_MINIMUM_SEVERITY_HELP}
          />
          <a
            href="/settings/system-relay"
            class="text-sm font-medium text-blue-600 dark:text-blue-400 hover:underline"
          >
            {ALERT_DESTINATIONS_PUSH_SETUP_LINK_LABEL} →
          </a>
        </div>
      </SettingsPanel>
    </Show>
  );
}
