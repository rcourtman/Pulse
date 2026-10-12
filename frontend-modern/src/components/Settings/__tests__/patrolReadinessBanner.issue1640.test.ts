// Regression coverage for issue #1640: an interrupted Patrol readiness run —
// an operator cancel, or a reverse proxy cutting a slow local evaluation —
// measured nothing about the model. The backend reports it as not assessed
// rather than a provider fault, and the settings banner must present it the
// same way. Rendering it in the red "Patrol model not verified" treatment is
// the blame-the-model presentation this fix exists to remove.
import { describe, expect, it } from 'vitest';
import {
  isPatrolReadinessUnassessed,
  patrolReadinessBannerHeadline,
  patrolReadinessBannerTone,
} from '../AIModelSelectionSection';
import type { PatrolModelReadinessSnapshot } from '@/types/ai';

const dimension = (status: PatrolModelReadinessSnapshot['status']) => ({ status, summary: '' });
const mode = () => ({ status: 'not_assessed' as const, summary: '' });

const snapshot = (
  overrides: Partial<PatrolModelReadinessSnapshot> = {},
): PatrolModelReadinessSnapshot => ({
  probe_version: 'patrol-readiness/v1',
  success: false,
  transport_healthy: false,
  patrol_capable: false,
  status: 'fail',
  provider: 'ollama',
  model: 'qwen3:4b',
  duration_ms: 1200,
  summary: '',
  dimensions: {
    connectivity: dimension('not_assessed'),
    tool_protocol: dimension('not_assessed'),
    context_quality: dimension('not_assessed'),
    latency: dimension('not_assessed'),
  },
  modes: { monitor: mode(), approval: mode(), assisted: mode(), full: mode() },
  recorded_at: '2026-07-28T10:00:00Z',
  recorded_at_unix: 1_785_232_800,
  ...overrides,
});

const noStale = { isStale: false, pendingModel: 'qwen3:4b', cachedModel: 'qwen3:4b' };

describe('Patrol readiness banner presentation (#1640)', () => {
  it('presents an interrupted run neutrally instead of as a model failure', () => {
    const interrupted = snapshot({
      status: 'not_assessed',
      cause: 'interrupted',
      transport_healthy: false,
      summary: 'Analysis interrupted before completion.',
    });

    expect(isPatrolReadinessUnassessed(interrupted)).toBe(true);
    expect(patrolReadinessBannerTone(interrupted, false)).toBe('neutral');

    const headline = patrolReadinessBannerHeadline(interrupted, noStale);
    expect(headline).toBe('Patrol model check did not complete');
    expect(headline).not.toContain('not verified');
  });

  it('presents any not_assessed run neutrally, whatever the cause', () => {
    const notAssessed = snapshot({ status: 'not_assessed', summary: 'Nothing was measured.' });
    expect(patrolReadinessBannerTone(notAssessed, false)).toBe('neutral');
    expect(patrolReadinessBannerHeadline(notAssessed, noStale)).toBe('Patrol model not assessed');
  });

  it('never claims verification for an interrupted run that had partial evidence', () => {
    // The interrupted run may still carry a max_verified_mode set before the
    // cancellation. Not assessed outranks it: the run never finished.
    const interrupted = snapshot({
      status: 'not_assessed',
      cause: 'interrupted',
      max_verified_mode: 'monitor',
    });
    expect(patrolReadinessBannerHeadline(interrupted, noStale)).toBe(
      'Patrol model check did not complete',
    );
    expect(patrolReadinessBannerTone(interrupted, false)).toBe('neutral');
  });

  it('keeps the existing presentation for real verdicts', () => {
    const passed = snapshot({
      status: 'pass',
      success: true,
      transport_healthy: true,
      patrol_capable: true,
      max_verified_mode: 'approval',
    });
    expect(patrolReadinessBannerTone(passed, false)).toBe('success');
    expect(patrolReadinessBannerHeadline(passed, noStale)).toBe(
      'Verified for Watch only and Ask first',
    );

    const failed = snapshot({ status: 'fail', cause: 'model_unsupported_tools' });
    expect(patrolReadinessBannerTone(failed, false)).toBe('error');
    expect(patrolReadinessBannerHeadline(failed, noStale)).toBe('Patrol model not verified');

    const warned = snapshot({ status: 'warning' });
    expect(patrolReadinessBannerTone(warned, false)).toBe('warning');
    expect(patrolReadinessBannerHeadline(warned, noStale)).toBe('Patrol model needs attention');

    const transportOnly = snapshot({
      status: 'fail',
      transport_healthy: true,
      patrol_capable: false,
    });
    expect(patrolReadinessBannerTone(transportOnly, false)).toBe('warning');
    expect(patrolReadinessBannerHeadline(transportOnly, noStale)).toBe(
      'Provider connected. Patrol capability not verified',
    );
  });

  it('keeps the stale-selection warning ahead of every other verdict', () => {
    const interrupted = snapshot({ status: 'not_assessed', cause: 'interrupted' });
    expect(patrolReadinessBannerTone(interrupted, true)).toBe('warning');
    expect(
      patrolReadinessBannerHeadline(interrupted, {
        isStale: true,
        pendingModel: 'qwen3:8b',
        cachedModel: 'qwen3:4b',
      }),
    ).toBe('Evaluation result is for qwen3:4b, your current selection is qwen3:8b');
  });

  it('stays idle with no result at all', () => {
    expect(patrolReadinessBannerTone(null, false)).toBe('idle');
    expect(patrolReadinessBannerTone(undefined, false)).toBe('idle');
    expect(patrolReadinessBannerHeadline(null, noStale)).toBe('');
  });
});
