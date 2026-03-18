export const ALERT_ADMINISTRATION_SECTION_TITLE = 'Administrative Actions';
export const ALERT_ADMINISTRATION_SECTION_DESCRIPTION =
  'Permanently clear all alert history. Use with caution - this action cannot be undone.';
export const ALERT_ADMINISTRATION_CLEAR_HISTORY_LABEL = 'Clear All History';
export const ALERT_ADMINISTRATION_CLEAR_HISTORY_ERROR =
  'Error clearing alert history: Please check your connection and try again.';
export const ALERT_ADMINISTRATION_CLEAR_HISTORY_CONFIRMATION =
  'Are you sure you want to clear all alert history?\n\nThis will permanently delete all historical alert data and cannot be undone.\n\nThis is typically only used for system maintenance or when starting fresh with a new monitoring setup.';

export function getAlertAdministrationSectionTitle() {
  return ALERT_ADMINISTRATION_SECTION_TITLE;
}

export function getAlertAdministrationSectionDescription() {
  return ALERT_ADMINISTRATION_SECTION_DESCRIPTION;
}

export function getAlertAdministrationClearHistoryLabel() {
  return ALERT_ADMINISTRATION_CLEAR_HISTORY_LABEL;
}

export function getAlertAdministrationClearHistoryError() {
  return ALERT_ADMINISTRATION_CLEAR_HISTORY_ERROR;
}

export function getAlertAdministrationClearHistoryConfirmation() {
  return ALERT_ADMINISTRATION_CLEAR_HISTORY_CONFIRMATION;
}
