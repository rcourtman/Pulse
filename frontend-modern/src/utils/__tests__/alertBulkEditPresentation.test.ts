import { describe, expect, it } from 'vitest';
import {
  ALERT_BULK_EDIT_CANCEL_LABEL,
  ALERT_BULK_EDIT_CLEAR_LABEL,
  ALERT_BULK_EDIT_DIALOG_TITLE,
  ALERT_BULK_EDIT_UNCHANGED_LABEL,
  getAlertBulkEditApplyLabel,
  getAlertBulkEditDescription,
  getAlertBulkEditOpenLabel,
} from '@/utils/alertBulkEditPresentation';

describe('alertBulkEditPresentation', () => {
  it('exposes canonical bulk-edit labels', () => {
    expect(ALERT_BULK_EDIT_DIALOG_TITLE).toBe('Bulk Edit Settings');
    expect(ALERT_BULK_EDIT_UNCHANGED_LABEL).toBe('Unchanged');
    expect(ALERT_BULK_EDIT_CLEAR_LABEL).toBe('Clear');
    expect(ALERT_BULK_EDIT_CANCEL_LABEL).toBe('Cancel');
  });

  it('formats canonical bulk-edit description and apply label', () => {
    expect(getAlertBulkEditDescription(4)).toBe(
      'Applying changes to 4 items. Leave fields empty to keep existing options.',
    );
    expect(getAlertBulkEditApplyLabel(4)).toBe('Apply to 4 items');
    expect(getAlertBulkEditOpenLabel()).toBe('Bulk Edit Settings');
  });
});
