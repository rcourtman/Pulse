import { describe, expect, it } from 'vitest';
import ts from 'typescript';
import alertsPageSource from '@/pages/Alerts.tsx?raw';
import activeAlertsSectionSource from '@/features/alerts/AlertOverviewActiveAlertsSection.tsx?raw';
import alertCardSource from '@/features/alerts/AlertOverviewAlertCard.tsx?raw';
import statsCardsSource from '@/features/alerts/AlertOverviewStatsCards.tsx?raw';
import acknowledgementStateSource from '@/features/alerts/useAlertAcknowledgementState.ts?raw';
import snoozeActionSource from '@/features/alerts/AlertSnoozeAction.tsx?raw';
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
  snoozeActionSource,
  incidentFiltersSource,
  incidentTimelineSource,
  investigateButtonSource,
  assistantHandoffSource,
] as const;

const migratedAlertsOverviewCopy = [
  'Notifications enabled',
  'Notifications paused',
  'Send notifications',
  'Alerts navigation',
  'Toggle external notifications',
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
  'More alert actions',
  'Explain with Assistant',
  'Use this alert as read-only context',
  'Have Patrol investigate',
  'Run a targeted Patrol check on this resource',
  'Have Patrol investigate this alert',
  'Alert investigation attached',
  'Diagnostics and remediation require operator approval',
  'No incident timeline available',
  'Filter events:',
  'Incident note',
  'Save Note',
  'Snooze this alert',
  'Pause notifications and escalation',
  'Until tomorrow at 9:00',
] as const;

// Check shipped copy, not explanatory comments which can legitimately name
// an action. Lexing preserves strings, including comment-like text within them.
function withoutComments(source: string): string {
  const scanner = ts.createScanner(ts.ScriptTarget.Latest, false, ts.LanguageVariant.JSX, source);
  const code: string[] = [];
  for (let token = scanner.scan(); token !== ts.SyntaxKind.EndOfFileToken; token = scanner.scan()) {
    if (
      token !== ts.SyntaxKind.SingleLineCommentTrivia &&
      token !== ts.SyntaxKind.MultiLineCommentTrivia
    ) {
      code.push(scanner.getTokenText());
    }
  }
  return code.join('');
}

describe('alerts overview localization guardrails', () => {
  it('ignores comments without excluding hardcoded string or JSX copy', () => {
    expect(
      withoutComments('// Acknowledge all\n/* Active Alerts */\nconst translated = label();'),
    ).not.toContain('Acknowledge all');
    expect(withoutComments('/* Active Alerts */\nconst translated = label();')).not.toContain(
      'Active Alerts',
    );
    expect(withoutComments('const label = "Acknowledge all"; // description')).toContain(
      'Acknowledge all',
    );
    expect(withoutComments('<button>Acknowledge all</button>')).toContain('Acknowledge all');
    expect(withoutComments('const label = "https://fixture.invalid/Acknowledge all";')).toContain(
      'Acknowledge all',
    );
    expect(withoutComments('const label = "/* Active Alerts */";')).toContain('Active Alerts');
  });

  it('prevents migrated alerts overview copy from reverting to hardcoded English', () => {
    for (const source of migratedAlertsSurfaceSources) {
      const code = withoutComments(source);
      for (const copy of migratedAlertsOverviewCopy) {
        expect(code).not.toContain(copy);
      }
    }

    expect(alertsPageSource).toContain('alertActivationPresentation().label');
    expect(activeAlertsSectionSource).toContain('getAlertOverviewActiveSectionTitle');
    expect(activeAlertsSectionSource).toContain('getAlertOverviewGroupDisclosureLabel');
    expect(alertCardSource).toContain('getAlertOverviewPrimaryActionLabel');
    expect(statsCardsSource).toContain('getAlertOverviewStatsLabels');
    expect(acknowledgementStateSource).toContain('getAlertOverviewAcknowledgedNotification');
    expect(investigateButtonSource).toContain('alerts.assistant.button.full');
    expect(investigateButtonSource).toContain('alerts.assistant.explain.menuLabel');
    expect(investigateButtonSource).toContain('alerts.assistant.patrol.title');
    expect(assistantHandoffSource).toContain('alerts.assistant.subject');
    expect(incidentTimelineSource).toContain('getAlertTimelineNotePlaceholder');
    expect(incidentFiltersSource).toContain('getAlertTimelineEventTypeLabel');
  });
});
