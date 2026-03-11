export const ALERT_BULK_EDIT_DIALOG_TITLE = 'Bulk Edit Settings';
export const ALERT_BULK_EDIT_UNCHANGED_LABEL = 'Unchanged';
export const ALERT_BULK_EDIT_CLEAR_LABEL = 'Clear';
export const ALERT_BULK_EDIT_CANCEL_LABEL = 'Cancel';

export function getAlertBulkEditDescription(selectedCount: number) {
  return `Applying changes to ${selectedCount} items. Leave fields empty to keep existing options.`;
}

export function getAlertBulkEditApplyLabel(selectedCount: number) {
  return `Apply to ${selectedCount} items`;
}

export function getAlertBulkEditOpenLabel() {
  return ALERT_BULK_EDIT_DIALOG_TITLE;
}
