import { describe, expect, it } from 'vitest';
import {
  formatAlertSeverityLabel,
  getAlertSeverityDetailTone,
  getAlertSeverityIndicator,
  getAlertSeverityBadgeClass,
  getAlertSeverityCompactLabel,
  getAlertSeverityDotClass,
  getAlertSeverityFillClass,
  getAlertSeverityIndicatorVariant,
  getAlertSeverityTextClass,
} from '@/utils/alertSeverityPresentation';

describe('alertSeverityPresentation', () => {
  it('formats provider severity labels predictably for platform alert rows', () => {
    expect(formatAlertSeverityLabel('critical')).toBe('Critical');
    expect(formatAlertSeverityLabel('restart_loop')).toBe('Restart Loop');
    expect(formatAlertSeverityLabel('')).toBe('Info');
  });

  it('maps alert severity buckets to status indicator variants', () => {
    expect(getAlertSeverityIndicatorVariant('critical')).toBe('danger');
    expect(getAlertSeverityIndicatorVariant('warning')).toBe('warning');
    expect(getAlertSeverityIndicatorVariant('info')).toBe('muted');
    expect(getAlertSeverityIndicatorVariant('maintenance')).toBe('muted');
  });

  it('uses bucket tone and provider label for shared alert indicators', () => {
    expect(getAlertSeverityIndicator('restart_loop', 'warning')).toEqual({
      variant: 'warning',
      label: 'Restart Loop',
    });
  });

  it('maps alert severity buckets to shared detail-row tones', () => {
    expect(getAlertSeverityDetailTone('critical')).toBe('danger');
    expect(getAlertSeverityDetailTone('warning')).toBe('warning');
    expect(getAlertSeverityDetailTone('info')).toBe('muted');
    expect(getAlertSeverityDetailTone('maintenance')).toBe('muted');
  });

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
    expect(getAlertSeverityDotClass('warning')).toBe(
      'h-2 w-2 rounded-full bg-amber-700 dark:bg-yellow-500',
    );
    expect(getAlertSeverityDotClass('info')).toBe('h-2 w-2 rounded-full bg-blue-500');
  });

  it('fills a severity the same colour as its dot, so a chart segment matches its legend', () => {
    for (const level of ['critical', 'warning', 'info', 'unknown']) {
      expect(getAlertSeverityDotClass(level)).toContain(getAlertSeverityFillClass(level));
    }
    expect(getAlertSeverityFillClass('CRITICAL')).toBe('bg-red-500');
  });

  it('maps compact severity labels canonically', () => {
    expect(getAlertSeverityCompactLabel('critical')).toBe('CRIT');
    expect(getAlertSeverityCompactLabel('warning')).toBe('WARN');
    expect(getAlertSeverityCompactLabel('info')).toBe('INFO');
  });
});
