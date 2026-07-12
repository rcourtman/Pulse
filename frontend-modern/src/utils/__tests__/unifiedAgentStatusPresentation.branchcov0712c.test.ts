import { describe, expect, it } from 'vitest';
import {
  getUnifiedAgentStatusPresentation,
  MONITORING_STOPPED_STATUS_LABEL,
} from '@/utils/unifiedAgentStatusPresentation';

const REMOVED_BADGE = 'bg-amber-100 text-amber-800 dark:bg-amber-900 dark:text-amber-200';
const CONNECTED_BADGE = 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-300';
const DISCONNECTED_BADGE = 'bg-surface-alt text-base-content';

describe('getUnifiedAgentStatusPresentation — branch coverage', () => {
  describe("state === 'removed' short-circuits before any health-status check", () => {
    it('takes precedence over a connected health status (online)', () => {
      expect(getUnifiedAgentStatusPresentation('removed', 'online')).toStrictEqual({
        badgeClass: REMOVED_BADGE,
        label: MONITORING_STOPPED_STATUS_LABEL,
      });
    });

    it('takes precedence over a disconnected health status (offline)', () => {
      expect(getUnifiedAgentStatusPresentation('removed', 'offline')).toStrictEqual({
        badgeClass: REMOVED_BADGE,
        label: MONITORING_STOPPED_STATUS_LABEL,
      });
    });

    it('applies when healthStatus is omitted', () => {
      expect(getUnifiedAgentStatusPresentation('removed')).toStrictEqual({
        badgeClass: REMOVED_BADGE,
        label: MONITORING_STOPPED_STATUS_LABEL,
      });
    });

    it('applies when healthStatus is null', () => {
      expect(getUnifiedAgentStatusPresentation('removed', null)).toStrictEqual({
        badgeClass: REMOVED_BADGE,
        label: MONITORING_STOPPED_STATUS_LABEL,
      });
    });
  });

  describe("state === 'active' with a connected health status (isConnectedHealthStatus === true)", () => {
    it("returns the green badge with the raw 'online' label", () => {
      expect(getUnifiedAgentStatusPresentation('active', 'online')).toStrictEqual({
        badgeClass: CONNECTED_BADGE,
        label: 'online',
      });
    });

    it("treats 'running' as connected", () => {
      expect(getUnifiedAgentStatusPresentation('active', 'running')).toStrictEqual({
        badgeClass: CONNECTED_BADGE,
        label: 'running',
      });
    });

    it("treats 'healthy' as connected", () => {
      expect(getUnifiedAgentStatusPresentation('active', 'healthy')).toStrictEqual({
        badgeClass: CONNECTED_BADGE,
        label: 'healthy',
      });
    });

    it('matches connectivity case-insensitively while preserving the raw label casing', () => {
      expect(getUnifiedAgentStatusPresentation('active', 'OnLiNe')).toStrictEqual({
        badgeClass: CONNECTED_BADGE,
        label: 'OnLiNe',
      });
    });

    it('trims surrounding whitespace for the connectivity check while keeping the raw label', () => {
      expect(getUnifiedAgentStatusPresentation('active', '  Running  ')).toStrictEqual({
        badgeClass: CONNECTED_BADGE,
        label: '  Running  ',
      });
    });
  });

  describe("state === 'active' with a non-connected health status (isConnectedHealthStatus === false)", () => {
    it("returns the muted badge and preserves a truthy non-connected status ('offline')", () => {
      expect(getUnifiedAgentStatusPresentation('active', 'offline')).toStrictEqual({
        badgeClass: DISCONNECTED_BADGE,
        label: 'offline',
      });
    });

    it("preserves a degraded status ('degraded') as the raw label", () => {
      expect(getUnifiedAgentStatusPresentation('active', 'degraded')).toStrictEqual({
        badgeClass: DISCONNECTED_BADGE,
        label: 'degraded',
      });
    });

    it("preserves the literal 'unknown' status as the label (truthy-fallback arm)", () => {
      expect(getUnifiedAgentStatusPresentation('active', 'unknown')).toStrictEqual({
        badgeClass: DISCONNECTED_BADGE,
        label: 'unknown',
      });
    });

    it('falls back to the "unknown" label when healthStatus is undefined', () => {
      expect(getUnifiedAgentStatusPresentation('active')).toStrictEqual({
        badgeClass: DISCONNECTED_BADGE,
        label: 'unknown',
      });
    });

    it('falls back to the "unknown" label when healthStatus is null', () => {
      expect(getUnifiedAgentStatusPresentation('active', null)).toStrictEqual({
        badgeClass: DISCONNECTED_BADGE,
        label: 'unknown',
      });
    });

    it('falls back to the "unknown" label when healthStatus is an empty string', () => {
      expect(getUnifiedAgentStatusPresentation('active', '')).toStrictEqual({
        badgeClass: DISCONNECTED_BADGE,
        label: 'unknown',
      });
    });
  });
});
