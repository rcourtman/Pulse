import { describe, expect, it } from 'vitest';
import {
  ALERT_THRESHOLDS_SECTION_DISABLED_LABEL,
  ALERT_THRESHOLDS_SECTION_UNSAVED_CHANGES_TITLE,
  getAlertThresholdsSectionDisabledLabel,
  getAlertThresholdsSectionUnsavedChangesTitle,
} from '@/utils/alertThresholdsSectionPresentation';

describe('alertThresholdsSectionPresentation', () => {
  it('returns canonical thresholds section status vocabulary', () => {
    expect(ALERT_THRESHOLDS_SECTION_DISABLED_LABEL).toBe('Disabled');
    expect(ALERT_THRESHOLDS_SECTION_UNSAVED_CHANGES_TITLE).toBe('Unsaved changes');
    expect(getAlertThresholdsSectionDisabledLabel()).toBe('Disabled');
    expect(getAlertThresholdsSectionUnsavedChangesTitle()).toBe('Unsaved changes');
  });
});
