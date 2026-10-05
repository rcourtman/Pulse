import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { DEFAULT_LOCALE, setActiveLocale } from '@/i18n';
import {
  getAlertFilteredEmptyState,
  getAlertOverviewAcknowledgedToggleLabel,
  getAlertOverviewAcknowledgementFailureNotification,
  getAlertOverviewBulkAcknowledgeFailureNotification,
  getAlertOverviewBulkAcknowledgedNotification,
  getAlertOverviewCardPresentation,
  getAlertOverviewTimelineActionLabel,
  getAlertTimelineEventTypeLabel,
} from '@/utils/alertOverviewPresentation';

describe('alertOverviewPresentation coverage', () => {
  beforeEach(() => {
    setActiveLocale(DEFAULT_LOCALE);
  });

  afterEach(() => {
    setActiveLocale(DEFAULT_LOCALE);
  });

  describe('severity projection — getAlertOverviewCardPresentation', () => {
    it('dims an acknowledged alert with opacity-60 when it is not processing', () => {
      const presentation = getAlertOverviewCardPresentation('critical', true, false);

      expect(presentation.cardClassName).toBe(
        'border border-l-4 rounded-md px-3 py-2.5 transition-all opacity-60 border-border border-l-border bg-surface-alt',
      );
      expect(presentation.iconClassName).toBe(
        'mr-2.5 mt-0.5 shrink-0 transition-all text-green-600 dark:text-green-400',
      );
      expect(presentation.resourceClassName).toBe(
        'text-sm font-semibold truncate text-base-content',
      );
    });

    it('projects an unacknowledged warning onto the warning palette', () => {
      const presentation = getAlertOverviewCardPresentation('warning', false, false);

      // Severity rides on the left edge and icon; the card itself stays neutral.
      expect(presentation.cardClassName).toBe(
        'border border-l-4 rounded-md px-3 py-2.5 transition-all border-border border-l-yellow-500 dark:border-l-yellow-400 bg-surface',
      );
      expect(presentation.iconClassName).toBe(
        'mr-2.5 mt-0.5 shrink-0 transition-all text-yellow-600 dark:text-yellow-400',
      );
      expect(presentation.resourceClassName).toBe(
        'text-sm font-semibold truncate text-base-content',
      );
    });

    it('projects an unacknowledged informational alert onto the informational palette', () => {
      const presentation = getAlertOverviewCardPresentation('info', false, false);

      expect(presentation.cardClassName).toContain('border-l-blue-500');
      expect(presentation.iconClassName).toContain('text-blue-600');
    });

    it('fails safe to warning for an unrecognized severity string', () => {
      const presentation = getAlertOverviewCardPresentation('notice', false, false);

      expect(presentation.cardClassName).toContain('border-l-yellow-500');
      expect(presentation.cardClassName).not.toContain('border-l-red-500');
      expect(presentation.iconClassName).toContain('text-yellow-600');
    });
  });

  describe('ack-state projection', () => {
    it('reports a restore failure when an already-acknowledged alert fails to toggle', () => {
      expect(getAlertOverviewAcknowledgementFailureNotification(true)).toBe(
        'Failed to restore alert',
      );
    });

    it('reports an acknowledge failure when an unacknowledged alert fails to toggle', () => {
      expect(getAlertOverviewAcknowledgementFailureNotification(false)).toBe(
        'Failed to acknowledge alert',
      );
    });

    it('offers to show acknowledged alerts when they are currently hidden', () => {
      expect(getAlertOverviewAcknowledgedToggleLabel(false)).toBe('Show acknowledged');
    });

    it('offers to collapse the timeline when it is expanded', () => {
      expect(getAlertOverviewTimelineActionLabel(true)).toBe('Hide Timeline');
    });
  });

  describe('bulk acknowledgement notification thresholds', () => {
    it('uses the singular success copy for a single acknowledged alert', () => {
      expect(getAlertOverviewBulkAcknowledgedNotification(1)).toBe('Acknowledged 1 alert.');
    });

    it('uses the plural failure copy when several alerts fail to acknowledge', () => {
      expect(getAlertOverviewBulkAcknowledgeFailureNotification(2)).toBe(
        'Failed to acknowledge 2 alerts.',
      );
    });
  });

  describe('timeline event-type projection', () => {
    it('labels an alert_fired event as Fired', () => {
      expect(getAlertTimelineEventTypeLabel('alert_fired')).toBe('Fired');
    });

    it('labels an alert_acknowledged event as Ack', () => {
      expect(getAlertTimelineEventTypeLabel('alert_acknowledged')).toBe('Ack');
    });

    it('labels an alert_unacknowledged event as Unack', () => {
      expect(getAlertTimelineEventTypeLabel('alert_unacknowledged')).toBe('Unack');
    });

    it('labels an ai_analysis event as Patrol', () => {
      expect(getAlertTimelineEventTypeLabel('ai_analysis')).toBe('Patrol');
    });

    it('labels a runbook event as Runbook', () => {
      expect(getAlertTimelineEventTypeLabel('runbook')).toBe('Runbook');
    });

    it('labels a note event as Note', () => {
      expect(getAlertTimelineEventTypeLabel('note')).toBe('Note');
    });
  });

  describe('filtered empty-state normalization', () => {
    it('falls back to the default subject and filter label with no arguments', () => {
      expect(getAlertFilteredEmptyState()).toEqual({
        title: 'No alerts match current filters',
        description: 'Adjust the search or active filter to see more alerts.',
      });
    });

    it('falls back to defaults when both arguments are only whitespace', () => {
      expect(getAlertFilteredEmptyState('   ', '  ')).toEqual({
        title: 'No alerts match current filters',
        description: 'Adjust the search or active filter to see more alerts.',
      });
    });

    it('keeps a provided subject but falls back to the default filter label', () => {
      expect(getAlertFilteredEmptyState('hosts', '   ')).toEqual({
        title: 'No hosts match current filters',
        description: 'Adjust the search or active filter to see more hosts.',
      });
    });
  });
});
