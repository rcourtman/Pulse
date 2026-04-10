import type { Component } from 'solid-js';
import {
  buildReleaseNotesUrl,
  buildV6RcFeedbackUrl,
  normalizeReleaseVersion,
} from '@/components/updateVersion';

export interface ReleaseCandidateBannerProps {
  version?: string | null;
}

export const ReleaseCandidateBanner: Component<ReleaseCandidateBannerProps> = (props) => {
  const versionLabel = () => normalizeReleaseVersion(props.version);
  const title = () =>
    versionLabel()
      ? `Pulse ${versionLabel()} is the first public v6 RC.`
      : 'You’re running a Pulse v6 release candidate build.';

  return (
    <div
      class="mb-3 rounded-md border border-amber-300 bg-amber-50 px-3 py-2 text-amber-950 dark:border-amber-800 dark:bg-amber-950/40 dark:text-amber-100"
      role="status"
      aria-live="polite"
    >
      <div class="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
        <div class="min-w-0">
          <div class="flex flex-wrap items-center gap-2">
            <span class="inline-flex rounded-full bg-amber-600 px-2 py-0.5 text-[10px] font-semibold uppercase tracking-[0.18em] text-white">
              RC
            </span>
            <span class="text-sm font-medium">{title()}</span>
          </div>
          <p class="mt-1 text-xs leading-relaxed text-amber-900/85 dark:text-amber-100/85">
            Start in a staging or non-critical environment first, then send feedback on bugs,
            regressions, or rough edges before general availability.
          </p>
        </div>
        <div class="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs font-medium">
          <a
            href={buildReleaseNotesUrl(props.version)}
            target="_blank"
            rel="noopener noreferrer"
            class="inline-flex min-h-9 items-center rounded px-1 py-1 underline underline-offset-2 hover:opacity-90"
          >
            View release notes
          </a>
          <a
            href={buildV6RcFeedbackUrl()}
            target="_blank"
            rel="noopener noreferrer"
            class="inline-flex min-h-9 items-center rounded px-1 py-1 underline underline-offset-2 hover:opacity-90"
          >
            Send feedback
          </a>
        </div>
      </div>
    </div>
  );
};
