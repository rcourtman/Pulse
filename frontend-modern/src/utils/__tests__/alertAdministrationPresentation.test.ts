import { describe, expect, it } from 'vitest';
import {
  ALERT_ADMINISTRATION_CLEAR_HISTORY_CONFIRMATION,
  ALERT_ADMINISTRATION_CLEAR_HISTORY_ERROR,
  ALERT_ADMINISTRATION_CLEAR_HISTORY_LABEL,
  ALERT_ADMINISTRATION_SECTION_DESCRIPTION,
  ALERT_ADMINISTRATION_SECTION_TITLE,
  getAlertAdministrationClearHistoryConfirmation,
  getAlertAdministrationClearHistoryError,
  getAlertAdministrationClearHistoryLabel,
  getAlertAdministrationSectionDescription,
  getAlertAdministrationSectionTitle,
} from '@/utils/alertAdministrationPresentation';

describe('alertAdministrationPresentation', () => {
  it('returns canonical alert administration copy', () => {
    expect(ALERT_ADMINISTRATION_SECTION_TITLE).toBe('Administrative Actions');
    expect(ALERT_ADMINISTRATION_SECTION_DESCRIPTION).toBe(
      'Permanently clear all alert history. Use with caution - this action cannot be undone.',
    );
    expect(ALERT_ADMINISTRATION_CLEAR_HISTORY_LABEL).toBe('Clear All History');
    expect(ALERT_ADMINISTRATION_CLEAR_HISTORY_ERROR).toBe(
      'Error clearing alert history: Please check your connection and try again.',
    );
    expect(ALERT_ADMINISTRATION_CLEAR_HISTORY_CONFIRMATION).toContain(
      'Are you sure you want to clear all alert history?',
    );
    expect(getAlertAdministrationSectionTitle()).toBe('Administrative Actions');
    expect(getAlertAdministrationSectionDescription()).toBe(
      'Permanently clear all alert history. Use with caution - this action cannot be undone.',
    );
    expect(getAlertAdministrationClearHistoryLabel()).toBe('Clear All History');
    expect(getAlertAdministrationClearHistoryError()).toBe(
      'Error clearing alert history: Please check your connection and try again.',
    );
    expect(getAlertAdministrationClearHistoryConfirmation()).toContain(
      'This will permanently delete all historical alert data and cannot be undone.',
    );
  });
});
