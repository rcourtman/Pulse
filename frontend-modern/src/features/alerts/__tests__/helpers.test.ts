import { describe, expect, it } from 'vitest';
import {
  clampMaxAlertsPerHour,
  fallbackMaxAlertsPerHour,
  getLocalTimezone,
  createDefaultQuietHours,
  createDefaultCooldown,
  createDefaultGrouping,
  createDefaultResolveNotifications,
  createDefaultAppriseConfig,
  createDefaultEmailConfig,
} from '@/features/alerts/helpers';

describe('alerts helpers', () => {
  describe('clampMaxAlertsPerHour', () => {
    it('returns default min for NaN', () => {
      expect(clampMaxAlertsPerHour(NaN)).toBe(1);
    });

    it('returns default min for undefined', () => {
      expect(clampMaxAlertsPerHour(undefined)).toBe(1);
    });

    it('returns default min for non-numeric', () => {
      expect(clampMaxAlertsPerHour('abc' as unknown as number)).toBe(1);
    });

    it('returns min for value below min', () => {
      expect(clampMaxAlertsPerHour(0)).toBe(1);
    });

    it('returns max for value above max', () => {
      expect(clampMaxAlertsPerHour(100)).toBe(10);
    });

    it('returns value when within range', () => {
      expect(clampMaxAlertsPerHour(5)).toBe(5);
    });

    it('handles negative values', () => {
      expect(clampMaxAlertsPerHour(-5)).toBe(1);
    });
  });

  describe('fallbackMaxAlertsPerHour', () => {
    it('returns default for NaN', () => {
      expect(fallbackMaxAlertsPerHour(NaN)).toBe(3);
    });

    it('returns default for undefined', () => {
      expect(fallbackMaxAlertsPerHour(undefined)).toBe(3);
    });

    it('returns default for 0', () => {
      expect(fallbackMaxAlertsPerHour(0)).toBe(3);
    });

    it('returns default for negative', () => {
      expect(fallbackMaxAlertsPerHour(-5)).toBe(3);
    });

    it('returns clamped value for positive', () => {
      expect(fallbackMaxAlertsPerHour(5)).toBe(5);
    });
  });

  describe('getLocalTimezone', () => {
    it('returns a timezone string', () => {
      const tz = getLocalTimezone();
      expect(typeof tz).toBe('string');
      expect(tz.length).toBeGreaterThan(0);
    });

    it('returns UTC as fallback', () => {
      const tz = getLocalTimezone();
      expect(tz).toMatch(/^[A-Za-z]+\/[A-Za-z_]+|UTC$/);
    });
  });

  describe('createDefaultQuietHours', () => {
    it('creates default quiet hours config', () => {
      const result = createDefaultQuietHours();
      
      expect(result.enabled).toBe(false);
      expect(result.start).toBe('22:00');
      expect(result.end).toBe('08:00');
      expect(result.timezone).toBe(getLocalTimezone());
    });

    it('has correct weekday defaults', () => {
      const result = createDefaultQuietHours();
      
      expect(result.days.monday).toBe(true);
      expect(result.days.tuesday).toBe(true);
      expect(result.days.wednesday).toBe(true);
      expect(result.days.thursday).toBe(true);
      expect(result.days.friday).toBe(true);
    });

    it('has correct weekend defaults', () => {
      const result = createDefaultQuietHours();
      
      expect(result.days.saturday).toBe(false);
      expect(result.days.sunday).toBe(false);
    });

    it('has correct suppress defaults', () => {
      const result = createDefaultQuietHours();
      
      expect(result.suppress.performance).toBe(false);
      expect(result.suppress.storage).toBe(false);
      expect(result.suppress.offline).toBe(false);
    });
  });

  describe('createDefaultCooldown', () => {
    it('creates default cooldown config', () => {
      const result = createDefaultCooldown();
      
      expect(result.enabled).toBe(true);
      expect(result.minutes).toBe(30);
      expect(result.maxAlerts).toBe(3);
    });
  });

  describe('createDefaultGrouping', () => {
    it('creates default grouping config', () => {
      const result = createDefaultGrouping();
      
      expect(result.enabled).toBe(true);
      expect(result.window).toBe(1);
      expect(result.byNode).toBe(true);
      expect(result.byGuest).toBe(false);
    });
  });

  describe('createDefaultResolveNotifications', () => {
    it('returns true by default', () => {
      expect(createDefaultResolveNotifications()).toBe(true);
    });
  });

  describe('createDefaultAppriseConfig', () => {
    it('creates default apprise config', () => {
      const result = createDefaultAppriseConfig();
      
      expect(result.enabled).toBe(false);
      expect(result.mode).toBe('cli');
      expect(result.targetsText).toBe('');
      expect(result.cliPath).toBe('apprise');
      expect(result.timeoutSeconds).toBe(15);
    });
  });

  describe('createDefaultEmailConfig', () => {
    it('creates default email config', () => {
      const result = createDefaultEmailConfig();
      
      expect(result.enabled).toBe(false);
      expect(result.from).toBe('');
      expect(result.to).toEqual([]);
    });
  });
});
