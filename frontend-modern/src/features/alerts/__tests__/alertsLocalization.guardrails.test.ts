import { describe, expect, it } from 'vitest';
import alertsPageSource from '@/pages/Alerts.tsx?raw';
import activeAlertsSectionSource from '@/features/alerts/AlertOverviewActiveAlertsSection.tsx?raw';
import alertCardSource from '@/features/alerts/AlertOverviewAlertCard.tsx?raw';
import statsCardsSource from '@/features/alerts/AlertOverviewStatsCards.tsx?raw';
import acknowledgementStateSource from '@/features/alerts/useAlertAcknowledgementState.ts?raw';
import incidentFiltersSource from '@/components/Alerts/IncidentEventFilters.tsx?raw';
import incidentTimelineSource from '@/components/Alerts/IncidentTimelinePanel.tsx?raw';
import investigateButtonSource from '@/components/Alerts/InvestigateAlertButton.tsx?raw';
import assistantHandoffSource from '@/components/Alerts/alertAssistantHandoffModel.ts?raw';

const migratedAlertsSurfaceSources = [
  alertsPageSource,
  activeAlertsSectionSource,
  alertCardSource,
  statsCardsSource,
  acknowledgementStateSource,
  incidentFiltersSource,
  incidentTimelineSource,
  investigateButtonSource,
  assistantHandoffSource,
] as const;

const migratedAlertsOverviewCopy = [
  'Alerts enabled',
  'Alerts disabled',
  'Alerts navigation',
  'Toggle alerts',
  'Active Alerts',
  'Alerting is paused',
  'Toggle alerts on to resume monitoring and unlock configuration tabs',
  'No active alerts',
  'No unacknowledged alerts',
  'Acknowledge all',
  'Acknowledging',
  'Unacknowledge',
  'Hide Timeline',
  'Triggered (24h)',
  'Workload Overrides',
  'Alert acknowledged',
  'Alert restored',
  'Failed to acknowledge alerts',
  'Ask Pulse Assistant about this alert',
  'Ask Pulse Assistant',
  'Pro required to ask Pulse Assistant about alerts',
  'More AI actions for this alert',
  'Have Patrol investigate',
  'Run a targeted Patrol check on this resource',
  'Alert investigation attached',
  'Diagnostics and remediation require operator approval',
  'No incident timeline available',
  'Filter events:',
  'Incident note',
  'Save Note',
] as const;

describe('alerts overview localization guardrails', () => {
  it('prevents migrated alerts overview copy from reverting to hardcoded English', () => {
    for (const source of migratedAlertsSurfaceSources) {
      for (const copy of migratedAlertsOverviewCopy) {
        expect(source).not.toContain(copy);
      }
    }

    expect(alertsPageSource).toContain('alerts.nav.toggleAlerts');
    expect(activeAlertsSectionSource).toContain('getAlertOverviewActiveSectionTitle');
    expect(alertCardSource).toContain('getAlertOverviewPrimaryActionLabel');
    expect(statsCardsSource).toContain('getAlertOverviewStatsLabels');
    expect(acknowledgementStateSource).toContain('getAlertOverviewAcknowledgedNotification');
    expect(investigateButtonSource).toContain('alerts.assistant.button.full');
    expect(assistantHandoffSource).toContain('alerts.assistant.subject');
    expect(incidentTimelineSource).toContain('getAlertTimelineNotePlaceholder');
    expect(incidentFiltersSource).toContain('getAlertTimelineEventTypeLabel');
  });
});
