import { Show } from 'solid-js';
import Smartphone from 'lucide-solid/icons/smartphone';
import { SettingsPanel } from '@/components/shared/SettingsPanel';
import { FeatureGateSection } from '@/components/shared/FeatureGateSection';
import type { UpgradeDestination } from '@/utils/upgradeNavigation';
import {
  ALERT_DESTINATIONS_PUSH_GATE_MESSAGE,
  ALERT_DESTINATIONS_PUSH_GATE_TITLE,
  ALERT_DESTINATIONS_PUSH_PANEL_DESCRIPTION,
  ALERT_DESTINATIONS_PUSH_PANEL_TITLE,
  ALERT_DESTINATIONS_PUSH_READY_MESSAGE,
  ALERT_DESTINATIONS_PUSH_SETUP_LINK_LABEL,
} from '@/utils/alertDestinationsPresentation';

interface AlertPushDestinationsSectionProps {
  relayLicensed: boolean;
  showUpgradePrompts: boolean;
  upgradeDestination: UpgradeDestination;
}

export function AlertPushDestinationsSection(props: AlertPushDestinationsSectionProps) {
  return (
    <SettingsPanel
      title={ALERT_DESTINATIONS_PUSH_PANEL_TITLE}
      description={ALERT_DESTINATIONS_PUSH_PANEL_DESCRIPTION}
      class="min-w-0"
      bodyClass=""
    >
      <Show
        when={props.relayLicensed}
        fallback={
          <FeatureGateSection
            icon={<Smartphone size={20} strokeWidth={2} />}
            title={ALERT_DESTINATIONS_PUSH_GATE_TITLE}
            body={ALERT_DESTINATIONS_PUSH_GATE_MESSAGE}
            upgradeDestination={props.upgradeDestination}
            showUpgradePrompts={props.showUpgradePrompts}
          />
        }
      >
        <div class="flex flex-col gap-2">
          <p class="text-sm text-muted">{ALERT_DESTINATIONS_PUSH_READY_MESSAGE}</p>
          <a
            href="/settings/system-relay"
            class="text-sm font-medium text-blue-600 dark:text-blue-400 hover:underline"
          >
            {ALERT_DESTINATIONS_PUSH_SETUP_LINK_LABEL} →
          </a>
        </div>
      </Show>
    </SettingsPanel>
  );
}
