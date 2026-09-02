import { For, Show, createMemo, createSignal, onCleanup, onMount } from 'solid-js';
import CalendarCheckIcon from 'lucide-solid/icons/calendar-check';
import RefreshIcon from 'lucide-solid/icons/refresh-cw';
import { getPatrolDigest, PATROL_DIGEST_DEFAULT_DAYS, type PatrolDigest } from '@/api/patrol';
import { Button, ButtonLink } from '@/components/shared/Button';
import { formatRelativeTime } from '@/utils/format';

// "This week" answers one question for a paying customer: what did Patrol do
// for me? Every tile is a rollup the backend already computed from records
// Pulse keeps (docs/PATROL_WEEKLY_DIGEST.md). The card shows only numbers the
// reader can act on; forensic detail stays in run history and Actions. The
// effective Patrol mode is stated once by the page header, so the card only
// carries mode-aware tile copy and never repeats that sentence.

const REFRESH_INTERVAL_MS = 60_000;

const usdFormatter = new Intl.NumberFormat(undefined, {
  style: 'currency',
  currency: 'USD',
  minimumFractionDigits: 2,
  maximumFractionDigits: 2,
});

const INVESTIGATION_OUTCOME_COPY: Array<{ key: string; label: string }> = [
  { key: 'needs_attention', label: 'need you' },
  { key: 'fix_failed', label: 'fix failed' },
  { key: 'fix_verification_failed', label: 'fix not confirmed' },
  { key: 'cannot_fix', label: 'could not fix' },
  { key: 'timed_out', label: 'timed out' },
  { key: 'fix_queued', label: 'fix waiting for approval' },
  { key: 'fix_executed', label: 'fix run' },
  { key: 'fix_verification_unknown', label: 'fix run, result unknown' },
  { key: 'fix_verified', label: 'fixed and verified' },
  { key: 'resolved', label: 'resolved' },
  { key: 'fix_rejected', label: 'fix declined' },
];

const plural = (count: number, singular: string, pluralForm = `${singular}s`): string =>
  `${count} ${count === 1 ? singular : pluralForm}`;

const formatDigestError = (error: unknown): string =>
  error instanceof Error ? error.message : 'The weekly summary could not be loaded.';

const formatWindowDate = (iso: string): string => {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return '';
  return date.toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
};

export function describeDigestInvestigationOutcomes(
  byOutcome: Record<string, number>,
  limit = 2,
): string[] {
  const lines: string[] = [];
  for (const entry of INVESTIGATION_OUTCOME_COPY) {
    const count = byOutcome[entry.key] ?? 0;
    if (count > 0) lines.push(`${count} ${entry.label}`);
    if (lines.length >= limit) break;
  }
  return lines;
}

export function describeDigestOpenFindings(digest: PatrolDigest): string {
  const open = digest.findings.open_by_severity;
  const openTotal = open.critical + open.warning + open.watch + open.info;
  if (digest.findings.new === 0) return 'Nothing new was raised.';
  if (openTotal === 0) return 'All of them have since cleared.';
  const parts: string[] = [];
  if (open.critical > 0) parts.push(`${open.critical} critical`);
  if (open.warning > 0) parts.push(`${open.warning} warning`);
  const detail = parts.length > 0 ? ` (${parts.join(', ')})` : '';
  return `${openTotal} still open${detail}.`;
}

interface DigestTile {
  id: string;
  value: string;
  label: string;
  details: string[];
  tone?: 'default' | 'positive' | 'attention';
}

export function buildDigestTiles(digest: PatrolDigest): DigestTile[] {
  const { runs, findings, investigations, actions, alerts, spend } = digest;

  const runDetails = [
    `${plural(runs.checks, 'check')} across ${plural(runs.resources_covered, 'resource')}.`,
  ];
  if (alerts.reviewed > 0) runDetails.push(`${plural(alerts.reviewed, 'alert')} looked into.`);
  if (runs.failed > 0) runDetails.push(`${plural(runs.failed, 'run')} failed.`);
  if (runs.last_run_at) runDetails.push(`Last run ${formatRelativeTime(runs.last_run_at)}.`);

  const resolvedDetails: string[] = [];
  if (findings.auto_resolved > 0) {
    resolvedDetails.push(`${findings.auto_resolved} cleared by Patrol on its own.`);
  }
  if (findings.dismissed > 0) resolvedDetails.push(`${findings.dismissed} dismissed by you.`);
  if (findings.suppressed > 0) resolvedDetails.push(`${findings.suppressed} muted for good.`);
  if (resolvedDetails.length === 0) {
    resolvedDetails.push(
      findings.resolved > 0 ? 'Resolved by you.' : 'No issues were resolved this period.',
    );
  }

  const investigationDetails = describeDigestInvestigationOutcomes(investigations.by_outcome);
  if (investigationDetails.length === 0) {
    investigationDetails.push(
      digest.mode === 'monitor'
        ? 'Patrol is watch only, so it reports issues without investigating them.'
        : 'No issues needed a closer look.',
    );
  } else {
    investigationDetails[investigationDetails.length - 1] += '.';
    if (investigationDetails.length > 1) investigationDetails[0] += ',';
  }

  const actionDetails: string[] = [];
  if (actions.executed > 0) {
    actionDetails.push(`${actions.verified} of ${actions.executed} verified afterwards.`);
  }
  if (actions.failed > 0) actionDetails.push(`${plural(actions.failed, 'action')} failed.`);
  if (actions.rejected > 0) actionDetails.push(`${actions.rejected} declined by you.`);
  if (actionDetails.length === 0 && actions.pending === 0) {
    actionDetails.push(
      digest.mode === 'monitor'
        ? 'Patrol is watch only, so no fixes were proposed.'
        : 'No fixes were needed.',
    );
  }

  const spendDetails = [`${plural(spend.calls, 'model call')}.`];
  if (spend.calls > 0 && !spend.pricing_known) {
    spendDetails.push('Some calls used a model with no known price.');
  }

  return [
    { id: 'runs', value: String(runs.total), label: 'Patrol runs', details: runDetails },
    {
      id: 'new',
      value: String(findings.new),
      label: 'New issues',
      details: [describeDigestOpenFindings(digest)],
      tone:
        findings.open_by_severity.critical + findings.open_by_severity.warning > 0
          ? 'attention'
          : 'default',
    },
    {
      id: 'resolved',
      value: String(findings.resolved),
      label: 'Issues resolved',
      details: resolvedDetails,
      tone: findings.resolved > 0 ? 'positive' : 'default',
    },
    {
      id: 'investigated',
      value: String(investigations.total),
      label: 'Investigated',
      details: investigationDetails,
    },
    {
      id: 'actions',
      value: String(actions.executed),
      label: 'Fixes run',
      details: actionDetails,
      tone: actions.pending > 0 ? 'attention' : actions.executed > 0 ? 'positive' : 'default',
    },
    {
      id: 'spend',
      value: spend.calls > 0 ? usdFormatter.format(spend.estimated_usd) : usdFormatter.format(0),
      label: 'Estimated spend',
      details: spendDetails,
    },
  ];
}

export function PatrolWeeklyDigestCard() {
  const [digest, setDigest] = createSignal<PatrolDigest | null>(null);
  const [loading, setLoading] = createSignal(true);
  const [error, setError] = createSignal('');

  const load = async (quiet = false) => {
    if (!quiet) setLoading(true);
    try {
      setDigest(await getPatrolDigest(PATROL_DIGEST_DEFAULT_DAYS));
      setError('');
    } catch (cause) {
      setError(formatDigestError(cause));
    } finally {
      if (!quiet) setLoading(false);
    }
  };

  onMount(() => {
    void load();
    const refresh = () => {
      if (document.visibilityState === 'visible') void load(true);
    };
    const timer = window.setInterval(refresh, REFRESH_INTERVAL_MS);
    document.addEventListener('visibilitychange', refresh);
    onCleanup(() => {
      window.clearInterval(timer);
      document.removeEventListener('visibilitychange', refresh);
    });
  });

  const windowLabel = createMemo(() => {
    const current = digest();
    if (!current) return `Last ${PATROL_DIGEST_DEFAULT_DAYS} days`;
    if (!current.window.history_complete && current.window.history_since) {
      const since = formatWindowDate(current.window.history_since);
      return since
        ? `Since ${since} (older runs are no longer kept)`
        : `Last ${current.window.days} days`;
    }
    return `Last ${current.window.days} days`;
  });

  const tiles = createMemo(() => {
    const current = digest();
    return current ? buildDigestTiles(current) : [];
  });

  const pendingCount = createMemo(() => digest()?.actions.pending ?? 0);
  const hasRuns = createMemo(() => (digest()?.runs.total ?? 0) > 0);

  return (
    <section
      class="overflow-hidden rounded-lg border border-border bg-surface"
      aria-labelledby="patrol-weekly-digest-title"
    >
      <div class="flex items-start justify-between gap-3 border-b border-border px-4 py-4 sm:px-5">
        <div>
          <h2 id="patrol-weekly-digest-title" class="text-base font-semibold text-base-content">
            This week
          </h2>
          <p class="mt-1 max-w-3xl text-sm leading-5 text-muted">
            What Patrol did for you. {windowLabel()}.
          </p>
        </div>
        <Button
          variant="secondary"
          size="sm"
          class="gap-1.5"
          onClick={() => void load()}
          disabled={loading()}
          aria-label="Refresh this week's summary"
        >
          <RefreshIcon
            class={`h-4 w-4 ${loading() ? 'motion-safe:animate-spin' : ''}`}
            aria-hidden="true"
          />
          <span class="hidden sm:inline">Refresh</span>
        </Button>
      </div>

      <div aria-live="polite">
        <Show when={error()}>
          {(message) => (
            <div class="m-4 rounded-md border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-800 dark:border-red-900 dark:bg-red-950/30 dark:text-red-200">
              <p class="font-semibold">This week's summary is unavailable</p>
              <p class="mt-1 text-xs leading-5">{message()}</p>
            </div>
          )}
        </Show>

        <Show
          when={!loading() || digest()}
          fallback={<p class="px-4 py-8 text-center text-sm text-muted">Adding up this week…</p>}
        >
          <Show when={digest()}>
            {(current) => (
              <Show
                when={hasRuns()}
                fallback={
                  <div class="flex min-h-24 items-start gap-3 px-4 py-4 sm:px-5">
                    <CalendarCheckIcon
                      class="mt-0.5 h-5 w-5 shrink-0 text-muted"
                      aria-hidden="true"
                    />
                    <div>
                      <h3 class="text-sm font-semibold text-base-content">
                        Patrol has not run in the last {current().window.days} days
                      </h3>
                      <p class="mt-1 text-xs leading-5 text-muted">
                        Once Patrol runs, this card adds up what it checked, found, fixed, and cost
                        you.
                      </p>
                    </div>
                  </div>
                }
              >
                <dl class="grid divide-y divide-border sm:grid-cols-2 sm:divide-x sm:divide-y-0 xl:grid-cols-3 sm:[&>*:nth-child(2n+1)]:border-l-0 xl:[&>*:nth-child(2n+1)]:border-l xl:[&>*:nth-child(3n+1)]:border-l-0 sm:[&>*:nth-child(n+3)]:border-t xl:[&>*:nth-child(n+3)]:border-t-0 xl:[&>*:nth-child(n+4)]:border-t">
                  <For each={tiles()}>
                    {(tile) => (
                      <div class="px-4 py-4 sm:px-5" data-digest-tile={tile.id}>
                        <dt class="text-xs font-semibold uppercase tracking-wide text-muted">
                          {tile.label}
                        </dt>
                        <dd
                          class={`mt-1 text-2xl font-semibold tabular-nums ${
                            tile.tone === 'attention'
                              ? 'text-amber-700 dark:text-amber-300'
                              : tile.tone === 'positive'
                                ? 'text-emerald-700 dark:text-emerald-300'
                                : 'text-base-content'
                          }`}
                        >
                          {tile.value}
                        </dd>
                        <dd class="mt-1 text-xs leading-5 text-muted">
                          <For each={tile.details}>
                            {(line) => <span class="block">{line}</span>}
                          </For>
                          <Show when={tile.id === 'actions' && pendingCount() > 0}>
                            <ButtonLink href="/actions" variant="secondary" size="sm" class="mt-2">
                              {plural(pendingCount(), 'fix')} waiting for your approval
                            </ButtonLink>
                          </Show>
                        </dd>
                      </div>
                    )}
                  </For>
                </dl>
              </Show>
            )}
          </Show>
        </Show>
      </div>
    </section>
  );
}

export default PatrolWeeklyDigestCard;
