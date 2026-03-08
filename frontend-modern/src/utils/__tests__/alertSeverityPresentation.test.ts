import { describe, expect, it } from 'vitest';
import {
  getAlertSeverityBadgeClass,
  getAlertSeverityTextClass,
} from '@/utils/alertSeverityPresentation';

describe('alertSeverityPresentation', () => {
  it('maps critical alerts to danger styling', () => {
    expect(getAlertSeverityBadgeClass('critical')).toContain('red-100');
  });

  it('maps warning alerts to warning styling', () => {
    expect(getAlertSeverityBadgeClass('warning')).toContain('amber-100');
  });

  it('maps unknown severities to informational styling', () => {
    expect(getAlertSeverityBadgeClass('info')).toContain('blue-100');
  });

  it('maps critical count text to danger styling', () => {
    expect(getAlertSeverityTextClass('critical')).toContain('text-red-600');
  });

  it('maps warning count text to warning styling', () => {
    expect(getAlertSeverityTextClass('warning')).toContain('text-amber-600');
  });
});
