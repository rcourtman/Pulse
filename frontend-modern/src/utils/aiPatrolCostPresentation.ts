import type {
  ModelInfo,
  PatrolCostProjection,
  PatrolModelGuidanceLevel,
  PatrolModelGuidanceResponse,
  PatrolModelGuidanceRule,
} from '@/types/ai';
import { getProviderFromModelId } from '@/utils/aiProviderPresentation';

/**
 * Presentation for the Patrol cost preview and model guidance markers.
 *
 * Vocabulary is pinned at a home-lab user who has never bought API credits:
 * say what a token is once, say what the number means for their bill, and
 * keep every figure labelled as an estimate with its assumption visible.
 */

export const PATROL_TOKEN_EXPLAINER =
  'Cloud providers charge per token (roughly three quarters of a word). Each Patrol run sends the model a snapshot of your infrastructure, so run size and how often it runs decide the bill.';

export const PATROL_COST_DEFAULT_INTERVAL_MINUTES = 360;

export type PatrolCostTone = 'neutral' | 'positive' | 'warning' | 'danger';

export interface PatrolCostPresentation {
  tone: PatrolCostTone;
  headline: string;
  detail: string;
  assumption: string;
  budget: string;
  budgetTone: PatrolCostTone;
  schedule: string;
}

export function formatPatrolCostUSD(usd: number): string {
  if (!Number.isFinite(usd) || usd <= 0) return '$0';
  if (usd < 0.01) return 'under $0.01';
  if (usd < 1) return `$${usd.toFixed(2)}`;
  if (usd < 100) return `$${usd.toFixed(2).replace(/\.00$/, '')}`;
  return `$${Math.round(usd).toLocaleString()}`;
}

export function formatPatrolTokenCount(tokens: number): string {
  if (!Number.isFinite(tokens) || tokens <= 0) return '0';
  if (tokens >= 1_000_000) return `${(tokens / 1_000_000).toFixed(1).replace(/\.0$/, '')}M`;
  if (tokens >= 1_000) return `${Math.round(tokens / 1_000)}k`;
  return String(Math.round(tokens));
}

export function formatPatrolIntervalPhrase(minutes: number): string {
  if (!Number.isFinite(minutes) || minutes <= 0) return 'manual runs only';
  if (minutes === 60) return 'every hour';
  if (minutes < 60) return `every ${minutes} minutes`;
  if (minutes === 1440) return 'once a day';
  if (minutes % 60 === 0) return `every ${minutes / 60} hours`;
  return `every ${minutes} minutes`;
}

function formatRunsPerDay(runsPerDay: number): string {
  if (!Number.isFinite(runsPerDay) || runsPerDay <= 0) return 'no scheduled runs';
  if (runsPerDay === 1) return '1 scheduled run a day';
  if (runsPerDay < 1) {
    const rounded = Math.round(runsPerDay * 10) / 10;
    return `${rounded} scheduled runs a day`;
  }
  return `${Math.round(runsPerDay)} scheduled runs a day`;
}

function intervalEstimate(projection: PatrolCostProjection, minutes: number): number | null {
  const match = projection.interval_estimates?.find((entry) => entry.interval_minutes === minutes);
  return match ? match.projected_30d_usd : null;
}

/** "≈ $5.11/mo" for a schedule option, or '' when nothing is billed. */
export function getPatrolIntervalCostHint(
  projection: PatrolCostProjection | null | undefined,
  minutes: number,
): string {
  if (!projection || !projection.pricing_known || !projection.billed_per_token) return '';
  if (minutes <= 0) return 'no scheduled cost';
  const usd = intervalEstimate(projection, minutes);
  if (usd == null) return '';
  return `≈ ${formatPatrolCostUSD(usd)}/month`;
}

export function getPatrolCostPresentation(
  projection: PatrolCostProjection | null | undefined,
): PatrolCostPresentation | null {
  if (!projection || !projection.model) return null;
  const modelLabel = projection.model;
  const budgetSet = projection.budget_usd_30d > 0;
  const spendKnown = projection.spend_30d_known;

  let budget: string;
  let budgetTone: PatrolCostTone = 'neutral';
  if (projection.budget_reached && budgetSet) {
    budget = `Budget reached: about ${formatPatrolCostUSD(projection.spend_30d_usd)} of your ${formatPatrolCostUSD(projection.budget_usd_30d)} 30-day budget is spent, so Patrol is paused until you raise it or the 30-day window rolls on.`;
    budgetTone = 'danger';
  } else if (budgetSet) {
    const patrolShare =
      projection.patrol_spend_30d_usd > 0
        ? ` (${formatPatrolCostUSD(projection.patrol_spend_30d_usd)} of that was Patrol)`
        : '';
    budget = spendKnown
      ? `Spent so far: about ${formatPatrolCostUSD(projection.spend_30d_usd)} of your ${formatPatrolCostUSD(projection.budget_usd_30d)} 30-day budget${patrolShare}.`
      : `Nothing priced yet against your ${formatPatrolCostUSD(projection.budget_usd_30d)} 30-day budget.`;
    if (
      spendKnown &&
      projection.billed_per_token &&
      projection.spend_30d_usd + projection.projected_30d_usd > projection.budget_usd_30d
    ) {
      budget += ' At this pace Patrol would pass the budget this month and pause.';
      budgetTone = 'warning';
    }
  } else {
    budget = spendKnown
      ? `Spent so far: about ${formatPatrolCostUSD(projection.spend_30d_usd)} in the last 30 days. No 30-day budget is set, so Patrol will not stop for cost. Set one under Provider & Models to cap surprises.`
      : 'No 30-day budget is set, so Patrol will not stop for cost. Set one under Provider & Models to cap surprises.';
  }

  if (!projection.pricing_known) {
    return {
      tone: 'neutral',
      headline: 'No price on file for this model',
      detail: `Pulse cannot estimate the bill for ${modelLabel}. Check the provider's pricing page. Pulse still counts the tokens each run uses.`,
      assumption: PATROL_TOKEN_EXPLAINER,
      budget,
      budgetTone,
      schedule: '',
    };
  }

  if (!projection.billed_per_token) {
    return {
      tone: 'positive',
      headline: 'No per-token bill for Patrol',
      detail: `${modelLabel} runs on your own hardware or a free route, so Patrol adds nothing to a provider bill. Only the schedule's load on that hardware changes.`,
      assumption: '',
      budget,
      budgetTone,
      schedule: '',
    };
  }

  const perRun = formatPatrolCostUSD(projection.per_run_usd);
  const rates = `${formatPatrolCostUSD(projection.input_usd_per_mtok)} in / ${formatPatrolCostUSD(projection.output_usd_per_mtok)} out per million tokens`;
  const priceDate = projection.pricing_as_of ? `, prices as of ${projection.pricing_as_of}` : '';
  const perRunTokens = `about ${formatPatrolTokenCount(projection.per_run_input_tokens)} tokens in and ${formatPatrolTokenCount(projection.per_run_output_tokens)} out per run`;

  let headline: string;
  let detail: string;
  if (projection.interval_minutes <= 0) {
    headline = `About ${perRun} per manual Patrol run`;
    detail = `Manual runs only: ${perRunTokens} at ${rates}${priceDate}.`;
  } else {
    headline = `About ${formatPatrolCostUSD(projection.projected_30d_usd)} a month for Patrol`;
    detail = `${formatRunsPerDay(projection.scheduled_runs_per_day)} × ${perRunTokens} ≈ ${perRun} a run, at ${rates}${priceDate}.`;
  }
  if (projection.triggered_runs_per_day > 0) {
    const perDay = Math.round(projection.triggered_runs_per_day * 10) / 10;
    detail += ` Alert-triggered runs added about ${perDay} a day recently and are included.`;
  } else {
    detail += ' Alert-triggered runs add to this when alerts fire.';
  }

  const assumption =
    projection.per_run_source === 'history'
      ? `Run size is the median of your last ${projection.history_run_count} full Patrol runs. ${PATROL_TOKEN_EXPLAINER}`
      : `Run size is a full Patrol run measured on a real install, and installs with more resources send more. ${PATROL_TOKEN_EXPLAINER}`;

  let schedule = '';
  const recommended = projection.recommended_interval_minutes;
  const current = projection.interval_minutes;
  const target = formatPatrolCostUSD(projection.recommendation_target_usd);
  const budgetWord = budgetSet
    ? 'your budget'
    : `a ${formatPatrolCostUSD(projection.recommendation_target_usd * 2)} budget`;
  if (projection.recommendation_reason === 'exceeds_budget_share_even_daily') {
    const daily = intervalEstimate(projection, 1440);
    schedule = `Even once a day costs about ${formatPatrolCostUSD(daily ?? 0)} a month, above ${target} (half of ${budgetWord}). A cheaper model is the better lever.`;
  } else if (recommended > 0 && current > 0 && recommended > current) {
    const recommendedUSD = intervalEstimate(projection, recommended);
    const currentPhrase = formatPatrolIntervalPhrase(current);
    schedule = `Pulse suggests ${formatPatrolIntervalPhrase(recommended)} (about ${formatPatrolCostUSD(recommendedUSD ?? 0)} a month) to keep scheduled runs under ${target}, half of ${budgetWord}. ${currentPhrase.charAt(0).toUpperCase()}${currentPhrase.slice(1)} is about ${formatPatrolCostUSD(projection.scheduled_projected_30d_usd)} a month.`;
  }

  let tone: PatrolCostTone = 'neutral';
  if (budgetTone === 'danger') tone = 'danger';
  else if (
    budgetTone === 'warning' ||
    projection.recommendation_reason === 'exceeds_budget_share_even_daily'
  )
    tone = 'warning';

  return { tone, headline, detail, assumption, budget, budgetTone, schedule };
}

export interface PatrolIntervalAutoAdjust {
  from: number;
  to: number;
  sentence: string;
}

/**
 * When a per-token model is picked and the schedule is still at the 6-hour
 * default (both in the form and as saved), propose the cost model's slower
 * schedule. Installs that already chose a schedule are never touched.
 */
export function getPatrolIntervalAutoAdjust(
  projection: PatrolCostProjection | null | undefined,
  options: { currentIntervalMinutes: number; savedIntervalMinutes: number },
): PatrolIntervalAutoAdjust | null {
  if (!projection || !projection.pricing_known || !projection.billed_per_token) return null;
  const { currentIntervalMinutes, savedIntervalMinutes } = options;
  if (
    currentIntervalMinutes !== PATROL_COST_DEFAULT_INTERVAL_MINUTES ||
    savedIntervalMinutes !== PATROL_COST_DEFAULT_INTERVAL_MINUTES
  ) {
    return null;
  }
  const to = projection.recommended_interval_minutes;
  if (!to || to <= currentIntervalMinutes) return null;
  const before = intervalEstimate(projection, currentIntervalMinutes);
  const after = intervalEstimate(projection, to);
  const hours = Math.round(to / 60);
  const sentence = `Schedule set to ${formatPatrolIntervalPhrase(to)} because ${projection.model} bills per token: about ${formatPatrolCostUSD(after ?? 0)} a month instead of ${formatPatrolCostUSD(before ?? 0)} at ${formatPatrolIntervalPhrase(currentIntervalMinutes)}, at the cost of a quiet problem waiting up to ${hours} hours for a scheduled check (alert-triggered runs still fire at once). Change it under Patrol › Schedule.`;
  return { from: currentIntervalMinutes, to, sentence };
}

export interface PatrolModelAnnotation {
  level: PatrolModelGuidanceLevel;
  badge: string;
  note: string;
  tone: 'positive' | 'warning' | 'neutral';
}

const GUIDANCE_PRIORITY: Record<PatrolModelGuidanceLevel, number> = {
  verified: 4,
  caution: 3,
  recommended: 2,
  suggested: 1,
};

const GUIDANCE_BADGES: Record<PatrolModelGuidanceLevel, string> = {
  verified: 'Verified on this install',
  recommended: 'Recommended for Patrol',
  suggested: 'Suggested starting point',
  caution: 'Caution',
};

function isGuidanceLevel(value: string): value is PatrolModelGuidanceLevel {
  return value in GUIDANCE_PRIORITY;
}

export function splitModelRoute(modelId: string): { provider: string; model: string } {
  const trimmed = modelId.trim();
  const colon = trimmed.indexOf(':');
  if (colon > 0) {
    return { provider: trimmed.slice(0, colon), model: trimmed.slice(colon + 1) };
  }
  return { provider: getProviderFromModelId(trimmed), model: trimmed };
}

export function matchesPatrolModelGuidanceRule(
  rule: PatrolModelGuidanceRule,
  provider: string,
  model: string,
): boolean {
  if (rule.provider.toLowerCase() !== provider.trim().toLowerCase()) return false;
  const candidate = model.trim().toLowerCase();
  const prefix = rule.model_prefix.toLowerCase();
  if (rule.model_exact ? candidate !== prefix : !candidate.startsWith(prefix)) return false;
  for (const needle of rule.exclude ?? []) {
    const lower = needle.toLowerCase();
    if (lower.startsWith('!')) {
      if (!candidate.includes(lower.slice(1))) return false;
      continue;
    }
    if (candidate.includes(lower)) return false;
  }
  return true;
}

/**
 * Resolve guidance markers for a model list. The install's own readiness
 * pass outranks the static table; a caution outranks a suggestion.
 */
export function resolvePatrolModelGuidance(
  models: Pick<ModelInfo, 'id' | 'provider'>[],
  guidance: PatrolModelGuidanceResponse | null | undefined,
): Map<string, PatrolModelAnnotation> {
  const result = new Map<string, PatrolModelAnnotation>();
  if (!guidance) return result;
  const verified = guidance.verified ?? null;
  for (const entry of models) {
    const route = splitModelRoute(entry.id);
    const provider = entry.provider?.trim() || route.provider;
    let best: PatrolModelAnnotation | null = null;
    for (const rule of guidance.rules ?? []) {
      if (!isGuidanceLevel(rule.level)) continue;
      if (!matchesPatrolModelGuidanceRule(rule, provider, route.model)) continue;
      const candidate: PatrolModelAnnotation = {
        level: rule.level,
        badge: GUIDANCE_BADGES[rule.level],
        note: rule.reason,
        tone: rule.level === 'caution' ? 'warning' : 'positive',
      };
      if (!best || GUIDANCE_PRIORITY[candidate.level] > GUIDANCE_PRIORITY[best.level]) {
        best = candidate;
      }
    }
    if (
      verified &&
      verified.provider.toLowerCase() === provider.toLowerCase() &&
      verified.model === route.model
    ) {
      const mode =
        verified.max_verified_mode === 'approval' ? 'Watch only and Ask first' : 'Watch only';
      best = {
        level: 'verified',
        badge: GUIDANCE_BADGES.verified,
        note: `Passed Check Patrol model here for ${mode}.${best?.level === 'caution' ? ` ${best.note}` : ''}`,
        tone: 'positive',
      };
    }
    if (best) result.set(entry.id, best);
  }
  return result;
}

/** Model ids to pin at the top of a picker, best guidance first. */
export function getPatrolGuidedModelIds(annotations: Map<string, PatrolModelAnnotation>): string[] {
  return Array.from(annotations.entries())
    .filter(([, annotation]) => annotation.level !== 'caution')
    .sort(([, a], [, b]) => GUIDANCE_PRIORITY[b.level] - GUIDANCE_PRIORITY[a.level])
    .map(([id]) => id);
}
