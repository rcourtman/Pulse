import { describe, expect, it } from 'vitest';
import { formatDiscoveryAge, getCategoryDisplayName, getConfidenceLevel } from '@/api/discovery';

describe('discovery utilities', () => {
  describe('formatDiscoveryAge', () => {
    it('returns Unknown for empty string', () => {
      expect(formatDiscoveryAge('')).toBe('Unknown');
    });

    it('returns NaN for invalid date', () => {
      const result = formatDiscoveryAge('invalid-date');
      expect(result).toContain('NaN');
    });

    it('returns Just now for very recent times', () => {
      const now = new Date().toISOString();
      expect(formatDiscoveryAge(now)).toBe('Just now');
    });

    it('returns minutes ago correctly', () => {
      const fiveMinutesAgo = new Date(Date.now() - 5 * 60 * 1000).toISOString();
      expect(formatDiscoveryAge(fiveMinutesAgo)).toBe('5 minutes ago');
    });

    it('returns 1 minute ago for exactly 1 minute', () => {
      const oneMinuteAgo = new Date(Date.now() - 60 * 1000).toISOString();
      expect(formatDiscoveryAge(oneMinuteAgo)).toBe('1 minute ago');
    });

    it('returns hours ago correctly', () => {
      const twoHoursAgo = new Date(Date.now() - 2 * 60 * 60 * 1000).toISOString();
      expect(formatDiscoveryAge(twoHoursAgo)).toBe('2 hours ago');
    });

    it('returns 1 hour ago for exactly 1 hour', () => {
      const oneHourAgo = new Date(Date.now() - 60 * 60 * 1000).toISOString();
      expect(formatDiscoveryAge(oneHourAgo)).toBe('1 hour ago');
    });

    it('returns days ago correctly', () => {
      const twoDaysAgo = new Date(Date.now() - 2 * 24 * 60 * 60 * 1000).toISOString();
      expect(formatDiscoveryAge(twoDaysAgo)).toBe('2 days ago');
    });

    it('returns 1 day ago for exactly 1 day', () => {
      const oneDayAgo = new Date(Date.now() - 24 * 60 * 60 * 1000).toISOString();
      expect(formatDiscoveryAge(oneDayAgo)).toBe('1 day ago');
    });
  });

  describe('getCategoryDisplayName', () => {
    it('returns correct name for database', () => {
      expect(getCategoryDisplayName('database')).toBe('Database');
    });

    it('returns correct name for web_server', () => {
      expect(getCategoryDisplayName('web_server')).toBe('Web Server');
    });

    it('returns correct name for cache', () => {
      expect(getCategoryDisplayName('cache')).toBe('Cache');
    });

    it('returns correct name for message_queue', () => {
      expect(getCategoryDisplayName('message_queue')).toBe('Message Queue');
    });

    it('returns correct name for monitoring', () => {
      expect(getCategoryDisplayName('monitoring')).toBe('Monitoring');
    });

    it('returns correct name for backup', () => {
      expect(getCategoryDisplayName('backup')).toBe('Backup');
    });

    it('returns correct name for nvr', () => {
      expect(getCategoryDisplayName('nvr')).toBe('NVR');
    });

    it('returns correct name for storage', () => {
      expect(getCategoryDisplayName('storage')).toBe('Storage');
    });

    it('returns correct name for container', () => {
      expect(getCategoryDisplayName('container')).toBe('Container');
    });

    it('returns correct name for virtualizer', () => {
      expect(getCategoryDisplayName('virtualizer')).toBe('Virtualizer');
    });

    it('returns correct name for network', () => {
      expect(getCategoryDisplayName('network')).toBe('Network');
    });

    it('returns correct name for security', () => {
      expect(getCategoryDisplayName('security')).toBe('Security');
    });

    it('returns correct name for media', () => {
      expect(getCategoryDisplayName('media')).toBe('Media');
    });

    it('returns correct name for home_automation', () => {
      expect(getCategoryDisplayName('home_automation')).toBe('Home Automation');
    });

    it('returns Unknown for unknown category', () => {
      expect(getCategoryDisplayName('unknown')).toBe('Unknown');
    });

    it('returns original string for unrecognized category', () => {
      expect(getCategoryDisplayName('custom_category')).toBe('custom_category');
    });
  });

  describe('getConfidenceLevel', () => {
    it('returns High confidence for >= 0.9', () => {
      const result = getConfidenceLevel(0.9);
      expect(result.label).toBe('High confidence');
      expect(result.color).toBe('text-green-600 dark:text-green-400');
    });

    it('returns High confidence for > 0.9', () => {
      const result = getConfidenceLevel(0.95);
      expect(result.label).toBe('High confidence');
    });

    it('returns Medium confidence for >= 0.7 and < 0.9', () => {
      const result = getConfidenceLevel(0.7);
      expect(result.label).toBe('Medium confidence');
      expect(result.color).toBe('text-amber-600 dark:text-amber-400');
    });

    it('returns Medium confidence for 0.8', () => {
      const result = getConfidenceLevel(0.8);
      expect(result.label).toBe('Medium confidence');
    });

    it('returns Low confidence for < 0.7', () => {
      const result = getConfidenceLevel(0.5);
      expect(result.label).toBe('Low confidence');
      expect(result.color).toBe('text-muted');
    });

    it('returns Low confidence for 0', () => {
      const result = getConfidenceLevel(0);
      expect(result.label).toBe('Low confidence');
    });

    it('returns Low confidence for negative values', () => {
      const result = getConfidenceLevel(-0.5);
      expect(result.label).toBe('Low confidence');
    });
  });
});
