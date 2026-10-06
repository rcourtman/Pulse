import { Show, splitProps, type Component, type JSX } from 'solid-js';

// warning: needs attention (amber). danger: down or unreachable (red).
// muted: no verdict, such as an unknown state.
export type PlatformIssueTone = 'warning' | 'danger' | 'muted';

export type PlatformIssue = {
  tone: PlatformIssueTone;
  // The row's bucket in words ("Attention", "Offline"), read out before the
  // reasons because sighted users get it from the colour.
  label: string;
  reasons: readonly string[];
};

const ISSUE_TONE_CLASS: Record<PlatformIssueTone, string> = {
  warning: 'text-amber-700 dark:text-amber-300',
  danger: 'text-red-600 dark:text-red-300',
  muted: 'text-muted',
};

// The visible reason is truncated and abbreviates the rest to a count, so
// assistive technology gets the bucket and every reason instead.
const issueScreenReaderText = (issue: PlatformIssue): string =>
  `${issue.label}: ${issue.reasons.map((reason) => reason.replace(/[.\s]+$/, '')).join('. ')}.`;

// A Health column cell that says why a row's status dot is not green.
// Healthy rows pass no issue and leave the cell empty: the dot already says
// healthy, and a column of green pills buries the few rows that are not. An
// exception shows its first reason on the row's single line (the shared
// platform-table rhythm), with every reason on hover and in the row drawer.
// The colour carries the bucket, so a pill repeating Attention would only take
// room from the reason.
export const PlatformIssueReason: Component<
  { issue: PlatformIssue | null | undefined } & Omit<
    JSX.HTMLAttributes<HTMLDivElement>,
    'class' | 'title' | 'children'
  >
> = (props) => {
  const [local, rest] = splitProps(props, ['issue']);
  return (
    <Show when={local.issue}>
      {(issue) => (
        <div
          {...rest}
          class={`flex min-w-0 items-center gap-1 text-[11px] font-medium ${ISSUE_TONE_CLASS[issue().tone]}`}
          title={issue().reasons.join('\n')}
        >
          <span aria-hidden="true" class="min-w-0 truncate">
            {issue().reasons[0] ?? issue().label}
          </span>
          <Show when={issue().reasons.length > 1}>
            <span aria-hidden="true" class="shrink-0 tabular-nums">
              +{issue().reasons.length - 1}
            </span>
          </Show>
          <span class="sr-only">{issueScreenReaderText(issue())}</span>
        </div>
      )}
    </Show>
  );
};
