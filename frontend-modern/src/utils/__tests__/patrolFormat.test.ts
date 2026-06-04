import { describe, expect, it } from 'vitest';
import {
  formatDurationMs,
  formatPatrolRuntimeFailureDetail,
  formatPatrolRuntimeFailureSummary,
  formatTriggerReason,
  formatScope,
  getCanonicalScopeResourceIds,
  sanitizeAnalysis,
  groupModelsByProvider,
} from '@/utils/patrolFormat';

describe('patrolFormat', () => {
  describe('formatDurationMs', () => {
    it('returns empty string for undefined', () => {
      expect(formatDurationMs(undefined)).toBe('');
    });

    it('returns empty string for 0', () => {
      expect(formatDurationMs(0)).toBe('');
    });

    it('returns empty string for negative', () => {
      expect(formatDurationMs(-100)).toBe('');
    });

    it('returns ms for less than 1 second', () => {
      expect(formatDurationMs(500)).toBe('500ms');
      expect(formatDurationMs(1)).toBe('1ms');
    });

    it('returns seconds for less than 60 seconds', () => {
      expect(formatDurationMs(1000)).toBe('1s');
      expect(formatDurationMs(55000)).toBe('55s');
    });

    it('returns minutes for 60 seconds or more', () => {
      expect(formatDurationMs(60000)).toBe('1m');
      expect(formatDurationMs(120000)).toBe('2m');
    });
  });

  describe('formatTriggerReason', () => {
    it('returns Scheduled for scheduled', () => {
      expect(formatTriggerReason('scheduled')).toBe('Scheduled');
    });

    it('returns Manual for manual', () => {
      expect(formatTriggerReason('manual')).toBe('Manual');
    });

    it('returns Startup for startup', () => {
      expect(formatTriggerReason('startup')).toBe('Startup');
    });

    it('returns Alert fired for alert_fired', () => {
      expect(formatTriggerReason('alert_fired')).toBe('Alert fired');
    });

    it('returns Alert cleared for alert_cleared', () => {
      expect(formatTriggerReason('alert_cleared')).toBe('Alert cleared');
    });

    it('returns Anomaly for anomaly', () => {
      expect(formatTriggerReason('anomaly')).toBe('Anomaly');
    });

    it('returns User action for user_action', () => {
      expect(formatTriggerReason('user_action')).toBe('User action');
    });

    it('returns Config change for config_changed', () => {
      expect(formatTriggerReason('config_changed')).toBe('Config change');
    });

    it('replaces underscores with spaces for unknown reasons', () => {
      expect(formatTriggerReason('some_reason')).toBe('some reason');
    });

    it('returns Unknown for undefined', () => {
      expect(formatTriggerReason(undefined)).toBe('Unknown');
    });

    it('returns Unknown for empty string', () => {
      expect(formatTriggerReason('')).toBe('Unknown');
    });
  });

  describe('formatScope', () => {
    it('returns undefined scope ids for an empty run', () => {
      expect(getCanonicalScopeResourceIds()).toBeUndefined();
    });

    it('returns effective scope ids when present', () => {
      expect(
        getCanonicalScopeResourceIds({
          scope_resource_ids: ['seed'],
          effective_scope_resource_ids: ['expanded-a', 'expanded-b'],
        }),
      ).toEqual(['expanded-a', 'expanded-b']);
    });

    it('preserves explicit empty effective scope ids', () => {
      expect(
        getCanonicalScopeResourceIds({
          scope_resource_ids: ['seed'],
          effective_scope_resource_ids: [],
        }),
      ).toEqual([]);
    });

    it('returns empty string for undefined', () => {
      expect(formatScope(undefined)).toBe('');
    });

    it('returns empty string for null', () => {
      expect(formatScope(null)).toBe('');
    });

    it('returns resource count for scope_resource_ids', () => {
      expect(formatScope({ scope_resource_ids: ['a', 'b'] })).toBe('Scoped to 2 resources');
    });

    it('returns singular for single resource', () => {
      expect(formatScope({ scope_resource_ids: ['a'] })).toBe('Scoped to 1 resource');
    });

    it('prefers effective_scope_resource_ids when present', () => {
      expect(
        formatScope({
          scope_resource_ids: ['seed'],
          effective_scope_resource_ids: ['seed', 'expanded'],
        }),
      ).toBe('Scoped to 2 resources');
    });

    it('returns type list for scope_resource_types', () => {
      expect(formatScope({ scope_resource_types: ['vm', 'container'] })).toBe(
        'Scoped to vm, container',
      );
    });

    it('returns Scoped for scoped type', () => {
      expect(formatScope({ type: 'scoped' })).toBe('Scoped');
    });

    it('prefers resource_ids over resource_types', () => {
      const result = formatScope({
        scope_resource_ids: ['a'],
        scope_resource_types: ['vm'],
      });
      expect(result).toBe('Scoped to 1 resource');
    });
  });

  describe('sanitizeAnalysis', () => {
    it('returns empty string for undefined', () => {
      expect(sanitizeAnalysis(undefined)).toBe('');
    });

    it('returns empty string for empty string', () => {
      expect(sanitizeAnalysis('')).toBe('');
    });

    it('removes DSML blocks', () => {
      const input = 'Some text<｜DSML｜model>content</｜DSML｜more>';
      expect(sanitizeAnalysis(input)).toBe('Some text');
    });

    it('removes standalone DSML tags', () => {
      const input = 'Text<｜DSML｜model>more';
      expect(sanitizeAnalysis(input)).toBe('Textmore');
    });

    it('trims whitespace', () => {
      expect(sanitizeAnalysis('  hello  ')).toBe('hello');
    });

    it('handles text without DSML', () => {
      expect(sanitizeAnalysis('Normal text')).toBe('Normal text');
    });
  });

  describe('formatPatrolRuntimeFailureSummary', () => {
    it('classifies provider billing payloads without exposing raw account details', () => {
      const summary = formatPatrolRuntimeFailureSummary({
        errorSummary: 'Provider billing or quota issue',
        errorDetail:
          'API error (402): {"error":{"message":"This request requires more credits. Visit https://openrouter.ai/settings/keys"},"user_id":"user_2rLolwcxpOSbgPEKJZuIxdW76pi"}',
        errorCount: 1,
      });

      expect(summary).toContain('Provider billing or quota issue');
      expect(summary).toContain('insufficient credits or token budget');
      expect(summary).not.toContain('openrouter.ai/settings/keys');
      expect(summary).not.toContain('user_2rLolwcxpOSbgPEKJZuIxdW76pi');
    });

    it('classifies tool-call failures without leaking provider routing payloads', () => {
      const summary = formatPatrolRuntimeFailureSummary({
        errorSummary: 'Selected model does not support Patrol tools',
        errorDetail: "No endpoints found that support the provided 'tool_choice' value.",
        errorCount: 1,
      });

      expect(summary).toContain('Selected model does not support Patrol tools');
      expect(summary).toContain('Provider rejected Patrol tool calls');
      expect(summary).not.toContain('tool_choice');
      expect(summary).not.toContain('No endpoints found');
    });

    it('redacts unknown provider details defensively', () => {
      const detail = formatPatrolRuntimeFailureDetail(
        'provider returned Authorization: Bearer sk-live-secret and url https://example.invalid/path?token=abc for user_abc123',
      );

      expect(detail).toContain('[redacted-url]');
      expect(detail).toContain('Bearer [redacted-secret]');
      expect(detail).toContain('[redacted-user]');
      expect(detail).not.toContain('sk-live-secret');
      expect(detail).not.toContain('token=abc');
      expect(detail).not.toContain('user_abc123');
    });
  });

  describe('groupModelsByProvider', () => {
    it('groups models by provider prefix', () => {
      const models = [
        { id: 'openai:gpt-4', name: 'GPT-4', description: '', notable: false },
        { id: 'openai:gpt-3.5', name: 'GPT-3.5', description: '', notable: false },
        { id: 'anthropic:claude', name: 'Claude', description: '', notable: false },
      ];

      const groups = groupModelsByProvider(models);
      expect(groups.get('openai')).toHaveLength(2);
      expect(groups.get('anthropic')).toHaveLength(1);
    });

    it('returns empty map for empty array', () => {
      const groups = groupModelsByProvider([]);
      expect(groups.size).toBe(0);
    });

    it('handles models without provider prefix', () => {
      const models = [{ id: 'no-provider', name: 'Model', description: '', notable: false }];

      const groups = groupModelsByProvider(models);
      expect(groups.get('no-provider')).toHaveLength(1);
    });

    it('prefers the server-supplied provider over the id prefix (#1320)', () => {
      const models = [
        // Opaque id (no recognizable provider prefix) but an explicit provider.
        { id: 'llama3-8b', name: 'Llama 3 8B', provider: 'ollama' },
        { id: 'qwen3.5-27b', name: 'Qwen', provider: 'ollama' },
      ];

      const groups = groupModelsByProvider(models);
      expect(groups.get('ollama')).toHaveLength(2);
      expect(groups.has('llama3-8b')).toBe(false);
    });
  });
});
