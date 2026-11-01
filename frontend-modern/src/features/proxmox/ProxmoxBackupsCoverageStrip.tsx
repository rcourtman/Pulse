import { For, Show, type Component, type JSX } from 'solid-js';
import ChevronRightIcon from 'lucide-solid/icons/chevron-right';
import { ProgressBar } from '@/components/shared/ProgressBar';
import { TableCard } from '@/components/shared/TableCard';
import { TableCardHeader } from '@/components/shared/TableCardHeader';
import type {
  BackupExceptionGroup,
  BackupExceptionTone,
  BackupVerdict,
  BackupVerdictTone,
} from './proxmoxBackupVerdict';

// The Backup health card answers "is everything backed up?" the way the
// Proxmox overview tables do: the protected count sits in the header strip,
// and each posture reason is one single-line row with a count pill and a bar
// for its share of rated targets. Each reason row toggles a Coverage filter
// to exactly those workloads; the card owns no filter state itself.

export interface CoverageStripProps {
  title: string;
  verdict: BackupVerdict;
  // Compact context shown on the right of the header strip.
  tail?: JSX.Element;
  // The rule behind the verdict. Collapsed by default: the counts are the
  // answer, and each workload's own drawer explains its state.
  rule?: JSX.Element;
  isGroupPressed?: (key: string) => boolean;
  onSelectGroup?: (key: string) => void;
}

const TONE_DOT_CLASS: Record<BackupVerdictTone, string> = {
  protected: 'bg-emerald-500',
  attention: 'bg-amber-500',
  unprotected: 'bg-red-500',
  unknown: 'bg-slate-400',
  checking: 'bg-blue-400',
};

type RowTone = BackupExceptionTone | 'protected';

const PILL_BASE =
  'inline-flex min-w-8 justify-center items-center rounded-md px-1.5 py-0.5 text-[11px] font-semibold tabular-nums';

const ROW_TONE: Record<RowTone, { dot: string; pill: string; fill: string }> = {
  unprotected: {
    dot: 'bg-red-500',
    pill: 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300',
    fill: 'bg-red-500/80',
  },
  attention: {
    dot: 'bg-amber-500',
    pill: 'bg-amber-100 text-amber-800 dark:bg-amber-900/40 dark:text-amber-300',
    fill: 'bg-amber-500/80',
  },
  unknown: {
    dot: 'bg-slate-400',
    pill: 'bg-surface-alt text-muted',
    fill: 'bg-slate-400/80',
  },
  protected: {
    dot: 'bg-emerald-500',
    pill: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300',
    fill: 'bg-emerald-500/80',
  },
};

const ROW_GRID =
  'grid w-full min-w-0 grid-cols-[0.5rem_minmax(0,1fr)_auto_5rem] items-center gap-x-3 px-3 text-xs sm:grid-cols-[0.5rem_minmax(0,16rem)_3rem_minmax(0,22rem)] sm:justify-start';

const PREVIEW_NAMES = 5;

// Names stay out of the row: a hover title previews them, and the filtered
// Coverage table lists them all.
const previewNames = (group: BackupExceptionGroup): string => {
  const names = group.rows.slice(0, PREVIEW_NAMES).map((row) => row.workload.label);
  const more = group.rows.length - names.length;
  return more > 0 ? `${names.join(', ')} and ${more} more` : names.join(', ');
};

const accessibleName = (group: BackupExceptionGroup): string =>
  `${group.rows.length} ${group.label.charAt(0).toLowerCase()}${group.label.slice(1)}`;

const percentOf = (count: number, total: number): number =>
  total > 0 ? Math.round((count / total) * 100) : 0;

const RowCells: Component<{ tone: RowTone; label: string; count: number; total: number }> = (
  props,
) => {
  const share = () => percentOf(props.count, props.total);
  return (
    <>
      <span class={`h-2 w-2 rounded-full ${ROW_TONE[props.tone].dot}`} aria-hidden="true" />
      <span class="min-w-0 truncate text-left text-base-content">{props.label}</span>
      <span class={`${PILL_BASE} ${ROW_TONE[props.tone].pill}`}>{props.count}</span>
      <ProgressBar
        value={share()}
        class="h-4"
        fillClass={ROW_TONE[props.tone].fill}
        label={
          <span class="absolute inset-0 flex items-center justify-center text-[10px] font-semibold leading-none text-base-content">
            {share()}%
          </span>
        }
      />
    </>
  );
};

export const ProxmoxBackupsCoverageStrip: Component<CoverageStripProps> = (props) => {
  const asides = () =>
    [
      props.verdict.checking > 0 ? `${props.verdict.checking} checking` : null,
      props.verdict.notEvaluated > 0 ? `${props.verdict.notEvaluated} not evaluated` : null,
    ].filter(Boolean) as string[];
  const rated = () => props.verdict.rated;

  return (
    <TableCard data-coverage-strip>
      <TableCardHeader
        title={
          <span class="inline-flex items-center gap-2 whitespace-nowrap">
            {props.title}
            <span
              class="inline-flex items-center gap-1.5 normal-case tracking-normal"
              data-backup-verdict={props.verdict.tone}
            >
              <span
                class={`h-2 w-2 shrink-0 rounded-full ${TONE_DOT_CLASS[props.verdict.tone]}`}
                aria-hidden="true"
              />
              <span class="sr-only">{props.verdict.headline}</span>
              <Show
                when={rated() > 0}
                fallback={
                  <span class="font-medium text-base-content" aria-hidden="true">
                    {props.verdict.headline}
                  </span>
                }
              >
                <span aria-hidden="true">
                  <span class="font-semibold tabular-nums text-base-content">
                    {props.verdict.protected} / {rated()}
                  </span>{' '}
                  <span class="font-medium">protected</span>
                </span>
              </Show>
              <Show when={asides().length > 0}>
                <span class="hidden font-medium text-muted sm:inline">
                  · {asides().join(' · ')}
                </span>
              </Show>
            </span>
          </span>
        }
        actions={
          <Show when={props.tail}>
            <span class="hidden text-[10px] font-medium text-muted sm:inline">{props.tail}</span>
          </Show>
        }
      />
      <Show when={rated() > 0}>
        <ul class="divide-y divide-border" aria-label="Backups that are not protected">
          <For each={props.verdict.exceptions}>
            {(group) => {
              const pressed = () => props.isGroupPressed?.(group.key) ?? false;
              return (
                <li class="flex" data-backup-exception={group.key}>
                  <Show
                    when={props.onSelectGroup}
                    fallback={
                      <span class={`${ROW_GRID} h-8`}>
                        <RowCells
                          tone={group.tone}
                          label={group.label}
                          count={group.rows.length}
                          total={rated()}
                        />
                      </span>
                    }
                  >
                    {(onSelect) => (
                      <button
                        type="button"
                        class={`${ROW_GRID} h-8 hover:bg-surface-hover focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-blue-500 ${
                          pressed() ? 'bg-blue-50 dark:bg-blue-900/30' : ''
                        }`}
                        aria-pressed={pressed()}
                        aria-label={accessibleName(group)}
                        title={previewNames(group)}
                        onClick={() => onSelect()(group.key)}
                      >
                        <RowCells
                          tone={group.tone}
                          label={group.label}
                          count={group.rows.length}
                          total={rated()}
                        />
                      </button>
                    )}
                  </Show>
                </li>
              );
            }}
          </For>
        </ul>
        <Show when={props.verdict.protected > 0}>
          <div class={`${ROW_GRID} h-8 border-t border-border`} data-backup-protected-row>
            <RowCells
              tone="protected"
              label="Protected"
              count={props.verdict.protected}
              total={rated()}
            />
          </div>
        </Show>
      </Show>
      <Show when={props.rule}>
        <details
          class="group border-t border-border px-3 py-1.5 text-[11px] text-muted"
          data-coverage-strip-rule
        >
          <summary class="inline-flex cursor-pointer list-none items-center gap-1 rounded-sm hover:text-base-content focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-blue-500 [&::-webkit-details-marker]:hidden">
            <ChevronRightIcon
              class="h-3 w-3 transition-transform group-open:rotate-90"
              aria-hidden="true"
            />
            What the counts mean
          </summary>
          <p class="mt-1 leading-4">{props.rule}</p>
        </details>
      </Show>
    </TableCard>
  );
};
