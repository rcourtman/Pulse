import { For, Show, type Component, type JSX } from 'solid-js';
import ChevronRightIcon from 'lucide-solid/icons/chevron-right';
import type {
  BackupExceptionGroup,
  BackupVerdict,
  BackupVerdictTone,
} from './proxmoxBackupVerdict';

// The Backup health strip answers "is everything backed up?" in one sentence,
// then names what is not: one line per posture reason with its count, so the
// answer never depends on counting rows. Each line toggles a Coverage filter
// to exactly those workloads; the strip owns no filter state itself.

export interface CoverageStripProps {
  title: string;
  verdict: BackupVerdict;
  // Compact context shown to the right of the title (e.g. "1.2 TB on disk").
  tail?: JSX.Element;
  // The rule behind the verdict. Collapsed by default: the sentence is the
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

const PREVIEW_NAMES = 3;

const previewNames = (group: BackupExceptionGroup): string => {
  const names = group.rows.slice(0, PREVIEW_NAMES).map((row) => row.workload.label);
  const more = group.rows.length - names.length;
  return more > 0 ? `${names.join(', ')} and ${more} more` : names.join(', ');
};

// The preview names are visual context; the button's name is its count and
// reason ("163 no successful backup"), which stays stable as names change.
const accessibleName = (group: BackupExceptionGroup): string =>
  `${group.rows.length} ${group.label.charAt(0).toLowerCase()}${group.label.slice(1)}`;

export const ProxmoxBackupsCoverageStrip: Component<CoverageStripProps> = (props) => {
  const asides = () =>
    [
      props.verdict.checking > 0 ? `${props.verdict.checking} checking` : null,
      props.verdict.notEvaluated > 0 ? `${props.verdict.notEvaluated} not evaluated` : null,
    ].filter(Boolean) as string[];

  return (
    <div class="rounded-lg border border-border-subtle px-3 py-2" data-coverage-strip>
      <div class="flex flex-wrap items-baseline gap-x-3 gap-y-1">
        <div class="text-[11px] font-semibold uppercase tracking-[0.18em] text-muted">
          {props.title}
        </div>
        <Show when={props.tail}>
          <div class="w-full text-[11px] text-muted sm:ml-auto sm:w-auto">{props.tail}</div>
        </Show>
      </div>
      <p
        class="mt-1.5 flex flex-wrap items-center gap-x-2 gap-y-0.5"
        data-backup-verdict={props.verdict.tone}
      >
        <span
          class={`h-2 w-2 shrink-0 rounded-full ${TONE_DOT_CLASS[props.verdict.tone]}`}
          aria-hidden="true"
        />
        <span class="text-sm font-medium text-base-content">{props.verdict.headline}</span>
        <Show when={asides().length > 0}>
          <span class="text-[11px] text-muted">{asides().join(' · ')}</span>
        </Show>
      </p>
      <Show when={props.verdict.exceptions.length > 0}>
        <ul class="mt-1 space-y-0.5" aria-label="Backups that are not protected">
          <For each={props.verdict.exceptions}>
            {(group) => {
              const pressed = () => props.isGroupPressed?.(group.key) ?? false;
              const content = () => (
                <>
                  <span
                    class={`h-2 w-2 shrink-0 rounded-xs ${TONE_DOT_CLASS[group.tone]}`}
                    aria-hidden="true"
                  />
                  <span class="min-w-0 truncate text-base-content">{group.label}</span>
                  <span class="shrink-0 tabular-nums text-muted">{group.rows.length}</span>
                  {/* Names help on a wide screen; a phone keeps the reason and count. */}
                  <span class="hidden min-w-0 flex-1 truncate text-muted sm:block">
                    {previewNames(group)}
                  </span>
                </>
              );
              return (
                <li class="flex" data-backup-exception={group.key}>
                  <Show
                    when={props.onSelectGroup}
                    fallback={
                      <span class="flex min-h-7 w-full min-w-0 items-center gap-2 px-1.5 text-xs sm:min-h-6">
                        {content()}
                      </span>
                    }
                  >
                    {(onSelect) => (
                      <button
                        type="button"
                        class={`flex min-h-7 w-full min-w-0 items-center gap-2 rounded-md border px-1.5 text-left text-xs hover:bg-surface-hover focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-blue-500 sm:min-h-6 ${
                          pressed()
                            ? 'border-blue-400 bg-blue-50 dark:border-blue-500 dark:bg-blue-900/30'
                            : 'border-transparent'
                        }`}
                        aria-pressed={pressed()}
                        aria-label={accessibleName(group)}
                        onClick={() => onSelect()(group.key)}
                      >
                        {content()}
                      </button>
                    )}
                  </Show>
                </li>
              );
            }}
          </For>
        </ul>
      </Show>
      <Show when={props.rule}>
        <details class="group mt-1 text-[11px] text-muted" data-coverage-strip-rule>
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
    </div>
  );
};

export default ProxmoxBackupsCoverageStrip;
