import { describe, expect, it } from 'vitest';
import {
  getAlertSeverityBadgeClass,
  getAlertSeverityCompactLabel,
  getAlertSeverityDotClass,
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

  it('maps severity dots to canonical colors', () => {
    expect(getAlertSeverityDotClass('critical')).toBe('h-2 w-2 rounded-full bg-red-500');
    expect(getAlertSeverityDotClass('warning')).toBe('h-2 w-2 rounded-full bg-yellow-500');
    expect(getAlertSeverityDotClass('info')).toBe('h-2 w-2 rounded-full bg-blue-500');
  });

  it('maps compact severity labels canonically', () => {
    expect(getAlertSeverityCompactLabel('critical')).toBe('CRIT');
    expect(getAlertSeverityCompactLabel('warning')).toBe('WARN');
    expect(getAlertSeverityCompactLabel('info')).toBe('INFO');
  });
});
