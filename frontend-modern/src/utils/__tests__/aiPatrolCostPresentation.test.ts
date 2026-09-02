import { describe, expect, it } from 'vitest';
import type { PatrolCostProjection, PatrolModelGuidanceResponse } from '@/types/ai';
import {
  formatPatrolCostUSD,
  formatPatrolIntervalPhrase,
  formatPatrolTokenCount,
  getPatrolCostPresentation,
  getPatrolGuidedModelIds,
  getPatrolIntervalAutoAdjust,
  getPatrolIntervalCostHint,
  resolvePatrolModelGuidance,
} from '@/utils/aiPatrolCostPresentation';

const flashProjection = (overrides: Partial<PatrolCostProjection> = {}): PatrolCostProjection => ({
  provider: 'gemini',
  model: 'gemini-2.5-flash',
  model_route: 'gemini:gemini-2.5-flash',
  billed_per_token: true,
  pricing_known: true,
  pricing_as_of: '2026-06-04',
  input_usd_per_mtok: 0.3,
  output_usd_per_mtok: 2.5,
  per_run_input_tokens: 104_528,
  per_run_output_tokens: 4_491,
  per_run_source: 'default',
  history_run_count: 0,
  per_run_usd: 0.0426,
  interval_minutes: 360,
  scheduled_runs_per_day: 4,
  triggered_runs_per_day: 0,
  triggered_per_run_usd: 0,
  scheduled_projected_30d_usd: 5.11,
  projected_30d_usd: 5.11,
  interval_estimates: [
    { interval_minutes: 60, scheduled_runs_per_day: 24, projected_30d_usd: 30.67 },
    { interval_minutes: 180, scheduled_runs_per_day: 8, projected_30d_usd: 10.22 },
    { interval_minutes: 360, scheduled_runs_per_day: 4, projected_30d_usd: 5.11 },
    { interval_minutes: 720, scheduled_runs_per_day: 2, projected_30d_usd: 2.56 },
    { interval_minutes: 1440, scheduled_runs_per_day: 1, projected_30d_usd: 1.28 },
  ],
  budget_usd_30d: 20,
  spend_30d_usd: 3.2,
  spend_30d_known: true,
  patrol_spend_30d_usd: 2.5,
  budget_reached: false,
  recommended_interval_minutes: 360,
  recommendation_reason: 'fits_budget_share',
  recommendation_target_usd: 10,
  ...overrides,
});

const sonnetProjection = (): PatrolCostProjection =>
  flashProjection({
    provider: 'anthropic',
    model: 'claude-sonnet-5',
    model_route: 'anthropic:claude-sonnet-5',
    input_usd_per_mtok: 2,
    output_usd_per_mtok: 10,
    per_run_usd: 0.254,
    scheduled_projected_30d_usd: 30.5,
    projected_30d_usd: 30.5,
    interval_estimates: [
      { interval_minutes: 60, scheduled_runs_per_day: 24, projected_30d_usd: 183 },
      { interval_minutes: 180, scheduled_runs_per_day: 8, projected_30d_usd: 61 },
      { interval_minutes: 360, scheduled_runs_per_day: 4, projected_30d_usd: 30.5 },
      { interval_minutes: 720, scheduled_runs_per_day: 2, projected_30d_usd: 15.25 },
      { interval_minutes: 1440, scheduled_runs_per_day: 1, projected_30d_usd: 7.62 },
    ],
    recommended_interval_minutes: 1440,
  });

describe('aiPatrolCostPresentation formatting', () => {
  it('formats dollars, tokens, and schedules the way a bill reads', () => {
    expect(formatPatrolCostUSD(0)).toBe('$0');
    expect(formatPatrolCostUSD(0.004)).toBe('under $0.01');
    expect(formatPatrolCostUSD(0.0426)).toBe('$0.04');
    expect(formatPatrolCostUSD(5.11)).toBe('$5.11');
    expect(formatPatrolCostUSD(20)).toBe('$20');
    expect(formatPatrolCostUSD(183)).toBe('$183');
    expect(formatPatrolTokenCount(104_528)).toBe('105k');
    expect(formatPatrolTokenCount(4_491)).toBe('4k');
    expect(formatPatrolTokenCount(2_400_000)).toBe('2.4M');
    expect(formatPatrolIntervalPhrase(0)).toBe('manual runs only');
    expect(formatPatrolIntervalPhrase(60)).toBe('every hour');
    expect(formatPatrolIntervalPhrase(360)).toBe('every 6 hours');
    expect(formatPatrolIntervalPhrase(1440)).toBe('once a day');
  });

  it('prices each schedule option only when something is billed', () => {
    expect(getPatrolIntervalCostHint(flashProjection(), 720)).toBe('≈ $2.56/month');
    expect(getPatrolIntervalCostHint(flashProjection(), 0)).toBe('no scheduled cost');
    expect(getPatrolIntervalCostHint(flashProjection({ billed_per_token: false }), 720)).toBe('');
    expect(getPatrolIntervalCostHint(null, 720)).toBe('');
  });
});

describe('getPatrolCostPresentation', () => {
  it('states the monthly estimate, the assumption, and spend against budget', () => {
    const presentation = getPatrolCostPresentation(flashProjection());
    expect(presentation).not.toBeNull();
    expect(presentation!.headline).toBe('About $5.11 a month for Patrol');
    expect(presentation!.detail).toBe(
      '4 scheduled runs a day × about 105k tokens in and 4k out per run ≈ $0.04 a run, at $0.30 in / $2.50 out per million tokens, prices as of 2026-06-04. Alert-triggered runs add to this when alerts fire.',
    );
    expect(presentation!.assumption).toContain('measured on a real install');
    expect(presentation!.assumption).toContain('roughly three quarters of a word');
    expect(presentation!.budget).toBe(
      'Spent so far: about $3.20 of your $20 30-day budget ($2.50 of that was Patrol).',
    );
    expect(presentation!.budgetTone).toBe('neutral');
    expect(presentation!.schedule).toBe('');
    expect(presentation!.tone).toBe('neutral');
  });

  it('uses the install history wording and counts triggered runs when present', () => {
    const presentation = getPatrolCostPresentation(
      flashProjection({
        per_run_source: 'history',
        history_run_count: 12,
        triggered_runs_per_day: 1.5,
        triggered_per_run_usd: 0.01,
        projected_30d_usd: 5.56,
      }),
    );
    expect(presentation!.headline).toBe('About $5.56 a month for Patrol');
    expect(presentation!.assumption).toContain('median of your last 12 full Patrol runs');
    expect(presentation!.detail).toContain(
      'Alert-triggered runs added about 1.5 a day recently and are included.',
    );
  });

  it('warns when the projection would pass the budget and flags a reached budget', () => {
    const nearLimit = getPatrolCostPresentation(flashProjection({ spend_30d_usd: 16 }));
    expect(nearLimit!.budget).toContain('At this pace Patrol would pass the budget this month');
    expect(nearLimit!.budgetTone).toBe('warning');
    expect(nearLimit!.tone).toBe('warning');

    const reached = getPatrolCostPresentation(
      flashProjection({ spend_30d_usd: 20.4, budget_reached: true }),
    );
    expect(reached!.budget).toBe(
      'Budget reached: about $20.40 of your $20 30-day budget is spent, so Patrol is paused until you raise it or the 30-day window rolls on.',
    );
    expect(reached!.tone).toBe('danger');
  });

  it('explains an unset budget and manual-only pricing', () => {
    const noBudget = getPatrolCostPresentation(
      flashProjection({ budget_usd_30d: 0, interval_minutes: 0, scheduled_runs_per_day: 0 }),
    );
    expect(noBudget!.headline).toBe('About $0.04 per manual Patrol run');
    expect(noBudget!.budget).toContain('No 30-day budget is set, so Patrol will not stop for cost');
  });

  it('recommends a slower schedule for expensive models and names the trade', () => {
    const presentation = getPatrolCostPresentation(sonnetProjection());
    expect(presentation!.headline).toBe('About $30.50 a month for Patrol');
    expect(presentation!.schedule).toBe(
      'Pulse suggests once a day (about $7.62 a month) to keep scheduled runs under $10, half of your budget. Every 6 hours is about $30.50 a month.',
    );
  });

  it('points at a cheaper model when even daily runs exceed the target', () => {
    const presentation = getPatrolCostPresentation(
      sonnetProjection() && {
        ...sonnetProjection(),
        budget_usd_30d: 0,
        recommendation_reason: 'exceeds_budget_share_even_daily',
        interval_estimates: [
          { interval_minutes: 1440, scheduled_runs_per_day: 1, projected_30d_usd: 38.1 },
        ],
      },
    );
    expect(presentation!.schedule).toBe(
      'Even once a day costs about $38.10 a month, above $10 (half of a $20 budget). A cheaper model is the better lever.',
    );
    expect(presentation!.tone).toBe('warning');
  });

  it('tells local and unpriced models apart from a bill', () => {
    const local = getPatrolCostPresentation(
      flashProjection({ provider: 'ollama', model: 'qwen3:8b', billed_per_token: false }),
    );
    expect(local!.headline).toBe('No per-token bill for Patrol');
    expect(local!.tone).toBe('positive');

    const unknown = getPatrolCostPresentation(
      flashProjection({ model: 'gpt-9-unpriced', pricing_known: false, billed_per_token: false }),
    );
    expect(unknown!.headline).toBe('No price on file for this model');
    expect(unknown!.detail).toContain('gpt-9-unpriced');
    expect(getPatrolCostPresentation(null)).toBeNull();
  });
});

describe('getPatrolIntervalAutoAdjust', () => {
  it('slows the default schedule for a per-token model and explains the trade-off', () => {
    const adjust = getPatrolIntervalAutoAdjust(sonnetProjection(), {
      currentIntervalMinutes: 360,
      savedIntervalMinutes: 360,
    });
    expect(adjust).toEqual({
      from: 360,
      to: 1440,
      sentence:
        'Schedule set to once a day because claude-sonnet-5 bills per token: about $7.62 a month instead of $30.50 at every 6 hours, at the cost of a quiet problem waiting up to 24 hours for a scheduled check (alert-triggered runs still fire at once). Change it under Patrol › Schedule.',
    });
  });

  it('leaves installs that already chose a schedule alone', () => {
    expect(
      getPatrolIntervalAutoAdjust(sonnetProjection(), {
        currentIntervalMinutes: 360,
        savedIntervalMinutes: 720,
      }),
    ).toBeNull();
    expect(
      getPatrolIntervalAutoAdjust(sonnetProjection(), {
        currentIntervalMinutes: 180,
        savedIntervalMinutes: 360,
      }),
    ).toBeNull();
  });

  it('does nothing for local models or when the default already fits', () => {
    expect(
      getPatrolIntervalAutoAdjust(flashProjection({ billed_per_token: false }), {
        currentIntervalMinutes: 360,
        savedIntervalMinutes: 360,
      }),
    ).toBeNull();
    expect(
      getPatrolIntervalAutoAdjust(flashProjection(), {
        currentIntervalMinutes: 360,
        savedIntervalMinutes: 360,
      }),
    ).toBeNull();
  });
});

describe('resolvePatrolModelGuidance', () => {
  const guidance: PatrolModelGuidanceResponse = {
    rules: [
      {
        provider: 'ollama',
        model_prefix: 'qwen3:8b',
        model_exact: true,
        level: 'recommended',
        reason: 'Passed the tool-call check.',
      },
      {
        provider: 'gemini',
        model_prefix: 'gemini-',
        exclude: ['!flash-lite'],
        level: 'caution',
        reason: 'Could not file verdicts.',
      },
      {
        provider: 'gemini',
        model_prefix: 'gemini-2.5-flash',
        exclude: ['lite'],
        level: 'suggested',
        reason: 'Lowest-cost standard tier.',
      },
    ],
    verified: {
      provider: 'ollama',
      model: 'qwen3:8b',
      max_verified_mode: 'approval',
      recorded_at_unix: 1,
    },
  };
  const models = [
    { id: 'ollama:qwen3:8b', provider: 'ollama' },
    { id: 'ollama:qwen3:4b', provider: 'ollama' },
    { id: 'gemini:gemini-2.5-flash', provider: 'gemini' },
    { id: 'gemini:gemini-2.5-flash-lite', provider: 'gemini' },
    { id: 'gemini:gemini-2.5-pro' },
  ];

  it('marks verified, suggested, and caution models and pins the good ones first', () => {
    const annotations = resolvePatrolModelGuidance(models, guidance);
    expect(annotations.get('ollama:qwen3:8b')).toEqual({
      level: 'verified',
      badge: 'Verified on this install',
      note: 'Passed Check Patrol model here for Watch only and Ask first.',
      tone: 'positive',
    });
    expect(annotations.get('ollama:qwen3:4b')).toBeUndefined();
    expect(annotations.get('gemini:gemini-2.5-flash')?.badge).toBe('Suggested starting point');
    expect(annotations.get('gemini:gemini-2.5-flash-lite')).toEqual({
      level: 'caution',
      badge: 'Caution',
      note: 'Could not file verdicts.',
      tone: 'warning',
    });
    expect(annotations.get('gemini:gemini-2.5-pro')).toBeUndefined();
    expect(getPatrolGuidedModelIds(annotations)).toEqual([
      'ollama:qwen3:8b',
      'gemini:gemini-2.5-flash',
    ]);
  });

  it('returns nothing without guidance', () => {
    expect(resolvePatrolModelGuidance(models, null).size).toBe(0);
  });
});
