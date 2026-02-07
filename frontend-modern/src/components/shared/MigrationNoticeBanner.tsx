import { Show, type Component } from 'solid-js';
import XIcon from 'lucide-solid/icons/x';

interface MigrationNoticeBannerProps {
  title: string;
  message: string;
  learnMoreHref?: string;
  onDismiss: () => void;
}

export const MigrationNoticeBanner: Component<MigrationNoticeBannerProps> = (props) => {
  return (
    <div class="rounded-lg border border-blue-200 bg-blue-50 px-3 py-2 text-blue-900 shadow-sm dark:border-blue-800/80 dark:bg-blue-950/40 dark:text-blue-100">
      <div class="flex items-start justify-between gap-3">
        <div class="min-w-0">
          <div class="text-xs font-semibold uppercase tracking-wide text-blue-700 dark:text-blue-300">
            Navigation update
          </div>
          <div class="mt-0.5 text-sm font-medium">{props.title}</div>
          <div class="mt-0.5 text-xs text-blue-800/90 dark:text-blue-200/90">{props.message}</div>
          <Show when={props.learnMoreHref}>
            <a
              href={props.learnMoreHref}
              class="mt-1 inline-flex text-xs font-medium text-blue-700 underline underline-offset-2 hover:text-blue-900 dark:text-blue-300 dark:hover:text-blue-100"
            >
              See full migration guide
            </a>
          </Show>
        </div>
        <button
          type="button"
          onClick={props.onDismiss}
          class="inline-flex h-6 w-6 items-center justify-center rounded-md text-blue-700 hover:bg-blue-100 hover:text-blue-900 dark:text-blue-300 dark:hover:bg-blue-900/40 dark:hover:text-blue-100"
          title="Dismiss"
          aria-label="Dismiss navigation notice"
        >
          <XIcon class="h-3.5 w-3.5" />
        </button>
      </div>
    </div>
  );
};

export default MigrationNoticeBanner;
