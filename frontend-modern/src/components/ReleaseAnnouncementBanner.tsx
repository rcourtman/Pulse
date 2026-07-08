import type { Accessor } from 'solid-js';
import { createMemo, Show } from 'solid-js';
import { useLocation } from '@solidjs/router';
import type { VersionInfo } from '@/api/updates';
import type { SecurityStatus } from '@/types/config';
import {
  V6_GA_ANNOUNCEMENT,
  shouldShowV6Announcement,
} from '@/constants/releaseAnnouncements';
import { createLocalStorageStringSignal, STORAGE_KEYS } from '@/utils/localStorage';
import InfoIcon from 'lucide-solid/icons/info';
import ExternalLinkIcon from 'lucide-solid/icons/external-link';
import XIcon from 'lucide-solid/icons/x';

interface ReleaseAnnouncementBannerProps {
  versionInfo: Accessor<VersionInfo | null>;
  securityStatus: Accessor<SecurityStatus | null>;
}

export function ReleaseAnnouncementBanner(props: ReleaseAnnouncementBannerProps) {
  const location = useLocation();
  const [dismissedAnnouncementId, setDismissedAnnouncementId] =
    createLocalStorageStringSignal(STORAGE_KEYS.RELEASE_ANNOUNCEMENT_DISMISSED, '');

  const isDismissed = createMemo(
    () => dismissedAnnouncementId() === V6_GA_ANNOUNCEMENT.id,
  );

  const shouldShow = createMemo(() => {
    if (isDismissed()) {
      return false;
    }

    return shouldShowV6Announcement({
      version: props.versionInfo()?.version,
      pathname: location.pathname,
      securityStatus: props.securityStatus(),
    });
  });

  const dismiss = () => {
    setDismissedAnnouncementId(V6_GA_ANNOUNCEMENT.id);
  };

  return (
    <Show when={shouldShow()}>
      <div class="border-b border-emerald-200 bg-emerald-50 text-emerald-950 dark:border-emerald-900/70 dark:bg-emerald-950/40 dark:text-emerald-100">
        <div class="px-4 py-3">
          <div class="flex items-start justify-between gap-3">
            <div class="flex min-w-0 flex-1 items-start gap-3">
              <div class="mt-0.5 rounded-full bg-emerald-100 p-2 text-emerald-700 dark:bg-emerald-900/70 dark:text-emerald-200">
                <InfoIcon class="h-4 w-4" />
              </div>
              <div class="min-w-0 flex-1">
                <div class="flex flex-wrap items-center gap-2">
                  <span class="text-sm font-semibold">Pulse v6 is available</span>
                </div>
                <p class="mt-1 text-sm leading-relaxed text-emerald-900/85 dark:text-emerald-100/85">
                  Pulse v6 is a new major version with a rebuilt interface and support for
                  more platforms. Installs on the{' '}
                  <code class="rounded bg-emerald-100 px-1 py-0.5 text-[12px] dark:bg-emerald-900/70">
                    5.1.x
                  </code>{' '}
                  line do not update to v6 automatically; upgrading is a manual step. See
                  the upgrade guide for what changes and how to move.
                </p>
                <div class="mt-3 flex flex-wrap items-center gap-2">
                  <a
                    href={V6_GA_ANNOUNCEMENT.upgradeGuideUrl}
                    target="_blank"
                    rel="noopener noreferrer"
                    class="inline-flex items-center gap-1 rounded-md bg-emerald-700 px-3 py-1.5 text-sm font-medium text-white transition-colors hover:bg-emerald-800 dark:bg-emerald-600 dark:hover:bg-emerald-500"
                  >
                    Upgrade guide
                    <ExternalLinkIcon class="h-3.5 w-3.5" />
                  </a>
                  <a
                    href={V6_GA_ANNOUNCEMENT.changelogUrl}
                    target="_blank"
                    rel="noopener noreferrer"
                    class="inline-flex items-center gap-1 rounded-md border border-emerald-300 bg-white px-3 py-1.5 text-sm font-medium text-emerald-900 transition-colors hover:bg-emerald-100 dark:border-emerald-800 dark:bg-emerald-950/20 dark:text-emerald-100 dark:hover:bg-emerald-900/40"
                  >
                    v6 changelog
                    <ExternalLinkIcon class="h-3.5 w-3.5" />
                  </a>
                </div>
              </div>
            </div>
            <button
              type="button"
              onClick={dismiss}
              class="rounded-md p-1 text-emerald-700 transition-colors hover:bg-emerald-100 hover:text-emerald-900 dark:text-emerald-300 dark:hover:bg-emerald-900/50 dark:hover:text-emerald-100"
              title="Dismiss"
              aria-label="Dismiss v6 announcement"
            >
              <XIcon class="h-4 w-4" />
            </button>
          </div>
        </div>
      </div>
    </Show>
  );
}
