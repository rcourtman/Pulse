import { describe, expect, it } from 'vitest';

import {
  buildPatrolSettingsReadinessFailure,
  resolvePatrolAutonomyLevelForSave,
  resolvePatrolAutonomySettingsForSave,
} from '../usePatrolIntelligenceState';
import type { AISettings, PatrolReadiness } from '@/types/ai';

const settingsWithReadiness = (patrolReadiness: PatrolReadiness): AISettings => ({
  enabled: true,
  model: 'ollama:llama3',
  configured: true,
  custom_context: '',
  auth_method: 'api_key',
  oauth_connected: false,
  anthropic_configured: false,
  openai_configured: false,
  openrouter_configured: false,
  deepseek_configured: false,
  gemini_configured: false,
  ollama_configured: true,
  ollama_base_url: 'http://127.0.0.1:11434',
  configured_providers: ['ollama'],
  patrol_readiness: patrolReadiness,
});

describe('usePatrolIntelligenceState', () => {
  describe('resolvePatrolAutonomyLevelForSave', () => {
    it('clamps stale paid autonomy to monitor when safe remediation is locked', () => {
      expect(resolvePatrolAutonomyLevelForSave('full', true, true)).toBe('monitor');
      expect(resolvePatrolAutonomyLevelForSave('assisted', false, true)).toBe('monitor');
      expect(resolvePatrolAutonomyLevelForSave('approval', false, true)).toBe('monitor');
    });

    it('preserves paid autonomy choices when safe remediation is available', () => {
      expect(resolvePatrolAutonomyLevelForSave('assisted', false, false)).toBe('assisted');
      expect(resolvePatrolAutonomyLevelForSave('assisted', true, false)).toBe('full');
      expect(resolvePatrolAutonomyLevelForSave('approval', false, false)).toBe('approval');
    });
  });

  describe('resolvePatrolAutonomySettingsForSave', () => {
    it('clears stale full-mode state when safe remediation is locked', () => {
      expect(
        resolvePatrolAutonomySettingsForSave({
          level: 'full',
          fullModeUnlocked: true,
          autoFixLocked: true,
        }),
      ).toEqual({ autonomyLevel: 'monitor', fullModeUnlocked: false });
    });

    it('does not carry full-mode state into non-remediation modes', () => {
      expect(
        resolvePatrolAutonomySettingsForSave({
          level: 'monitor',
          fullModeUnlocked: true,
          autoFixLocked: false,
        }),
      ).toEqual({ autonomyLevel: 'monitor', fullModeUnlocked: false });

      expect(
        resolvePatrolAutonomySettingsForSave({
          level: 'approval',
          fullModeUnlocked: true,
          autoFixLocked: false,
        }),
      ).toEqual({ autonomyLevel: 'approval', fullModeUnlocked: false });
    });

    it('promotes remediation mode to full only when full mode is explicitly unlocked', () => {
      expect(
        resolvePatrolAutonomySettingsForSave({
          level: 'assisted',
          fullModeUnlocked: true,
          autoFixLocked: false,
        }),
      ).toEqual({ autonomyLevel: 'full', fullModeUnlocked: true });

      expect(
        resolvePatrolAutonomySettingsForSave({
          level: 'full',
          fullModeUnlocked: false,
          autoFixLocked: false,
        }),
      ).toEqual({ autonomyLevel: 'assisted', fullModeUnlocked: false });
    });
  });

  describe('buildPatrolSettingsReadinessFailure', () => {
    it('ignores settings snapshots that do not block Patrol readiness', () => {
      expect(
        buildPatrolSettingsReadinessFailure({
          settings: settingsWithReadiness({
            status: 'warning',
            ready: true,
            summary: 'Patrol can run with reduced confidence.',
            checks: [],
          }),
        }),
      ).toBeNull();
    });

    it('builds a saved configuration issue from a not-ready settings response', () => {
      expect(
        buildPatrolSettingsReadinessFailure({
          settings: settingsWithReadiness({
            status: 'not_ready',
            ready: false,
            cause: 'model_unsupported_tools',
            summary: 'The selected model cannot run Patrol tools.',
            provider: 'ollama',
            model: 'ollama:deepseek-r1:7b',
            checks: [],
          }),
          autonomyLevel: 'monitor',
          fullModeUnlocked: false,
          investigationBudget: 15,
          investigationTimeoutSec: 300,
          runtimeState: 'blocked',
          blockedReason: 'Connect a tool-capable Patrol model.',
        }),
      ).toEqual({
        message: 'The selected model cannot run Patrol tools.',
        code: 'patrol_readiness_not_ready',
        status: 409,
        saved: true,
        details: {
          status: 'not_ready',
          cause: 'model_unsupported_tools',
          summary: 'The selected model cannot run Patrol tools.',
          provider: 'ollama',
          model: 'ollama:deepseek-r1:7b',
        },
        autonomyLevel: 'monitor',
        fullModeUnlocked: false,
        investigationBudget: 15,
        investigationTimeoutSec: 300,
        readiness: {
          status: 'not_ready',
          cause: 'model_unsupported_tools',
          summary: 'The selected model cannot run Patrol tools.',
          provider: 'ollama',
          model: 'ollama:deepseek-r1:7b',
        },
        runtimeState: 'blocked',
        blockedReason: 'Connect a tool-capable Patrol model.',
        blockedCause: undefined,
      });
    });
  });
});
