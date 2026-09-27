import { Show, createEffect, createSignal } from 'solid-js';
import CheckCircleIcon from 'lucide-solid/icons/check-circle';
import XIcon from 'lucide-solid/icons/x';
import { updateStore } from '@/stores/updates';
import { UpdatesAPI } from '@/api/updates';
import { reserveLowPriorityNoticeSession, STORAGE_KEYS } from '@/utils/localStorage';
import { ActionIconButton, Button } from '@/components/shared/Button';
import { Dialog } from '@/components/shared/Dialog';
import { ExternalTextLink } from '@/components/shared/ExternalTextLink';
import { InlineNotice } from '@/components/shared/InlineNotice';
import { buildReleaseNotesUrl, normalizeReleaseVersion } from '@/components/updateVersion';
import { extractChangelog, isReleaseVersion } from '@/components/whatsNewModel';
import { renderMarkdown } from '@/components/AI/aiChatUtils';
import { logger } from '@/utils/logger';

const readLastSeenVersion = (): string | null => {
  try {
    return localStorage.getItem(STORAGE_KEYS.WHATS_NEW_LAST_SEEN);
  } catch {
    return null;
  }
};

const markVersionSeen = (version: string) => {
  try {
    localStorage.setItem(STORAGE_KEYS.WHATS_NEW_LAST_SEEN, version);
  } catch {
    // Private mode / storage disabled: the dialog simply won't persist state.
  }
};

/**
 * Post-update "What's New" notice. A compact non-blocking notice appears once
 * after the running version changes and only when that release has categorized
 * user-facing changelog entries. The detailed dialog opens only after an
 * explicit action. Preparing the notice (or finding no categorized entries)
 * records the version so reloads stay quiet until the next update.
 *
 * Telemetry payload changes are not announced here. They are disclosed in
 * release notes and the dated "Payload changes" section of docs/PRIVACY.md;
 * the Settings payload preview always shows the exact current contract.
 */
export function WhatsNewCard() {
  const [noticeVisible, setNoticeVisible] = createSignal(false);
  const [dialogVisible, setDialogVisible] = createSignal(false);
  const [version, setVersion] = createSignal('');
  const [changelogHtml, setChangelogHtml] = createSignal('');
  let checked = false;

  const loadNotes = async (currentVersion: string, noticeSlotReserved: boolean) => {
    try {
      const notes = await UpdatesAPI.getReleaseNotes();
      // Mid-update the backend can briefly disagree with the UI about the
      // running version; don't show notes for the wrong release.
      if (normalizeReleaseVersion(notes.version) !== currentVersion) {
        return;
      }
      const changelog = extractChangelog(notes.releaseNotes);
      if (!changelog) {
        markVersionSeen(currentVersion);
        return;
      }
      setChangelogHtml(renderMarkdown(changelog));
      setVersion(currentVersion);
      // The compact notice is the only automatic release UI. Record the
      // version immediately so reloads cannot turn it into a recurring prompt.
      markVersionSeen(currentVersion);
      if (noticeSlotReserved) {
        setNoticeVisible(true);
      }
    } catch (error) {
      if ((error as { status?: number }).status === 404) {
        // No published release for this build — stop asking.
        markVersionSeen(currentVersion);
        return;
      }
      // Transient failure: leave last-seen untouched so the next load retries.
      logger.warn("Failed to load release notes for What's New dialog", error);
    }
  };

  createEffect(() => {
    const info = updateStore.versionInfo();
    if (!info || checked) return;
    checked = true;

    if (info.isDevelopment || info.isSourceBuild || !isReleaseVersion(info.version)) {
      return;
    }

    const currentVersion = normalizeReleaseVersion(info.version);
    if (!currentVersion) return;

    const lastSeen = readLastSeenVersion();
    if (!lastSeen) {
      // First run (fresh install or first load after this feature shipped):
      // record the baseline silently instead of greeting users with a dialog.
      markVersionSeen(currentVersion);
      return;
    }
    if (normalizeReleaseVersion(lastSeen) === currentVersion) {
      return;
    }

    const noticeSlotReserved = reserveLowPriorityNoticeSession('release-update');
    void loadNotes(currentVersion, noticeSlotReserved);
  });

  const dismissNotice = () => {
    setNoticeVisible(false);
  };

  const openChangelog = () => {
    setNoticeVisible(false);
    setDialogVisible(true);
  };

  const dismissChangelog = () => {
    setDialogVisible(false);
  };

  return (
    <>
      <Show when={noticeVisible()}>
        <aside
          class="fixed bottom-[var(--pulse-mobile-nav-height)] left-4 right-4 z-30 max-w-sm md:right-auto md:bottom-4"
          aria-live="polite"
          data-testid="whats-new-notice"
        >
          <InlineNotice
            tone="success"
            icon={<CheckCircleIcon class="h-4 w-4" aria-hidden="true" />}
            actionLabel="See what's new"
            actionOnClick={openChangelog}
            actionAriaLabel={`See what's new in Pulse ${version()}`}
            onDismiss={dismissNotice}
            dismissLabel="Dismiss update notice"
            dismissTitle="Dismiss"
            class="shadow-lg"
          >
            <span class="font-semibold">Pulse updated to v{version()}</span>
          </InlineNotice>
        </aside>
      </Show>

      <Show when={dialogVisible()}>
        <Dialog
          isOpen={dialogVisible()}
          onClose={dismissChangelog}
          panelClass="max-w-xl"
          ariaLabelledBy="whats-new-title"
        >
          <div class="w-full" data-testid="whats-new-modal">
            <div class="px-6 py-4 border-b border-border">
              <div class="flex items-center justify-between gap-3">
                <div class="flex items-center gap-3 min-w-0">
                  {/* Sparkle icon */}
                  <svg
                    class="w-5 h-5 flex-shrink-0 text-emerald-600 dark:text-emerald-400"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="2"
                    aria-hidden="true"
                  >
                    <path
                      d="M12 3l1.9 5.1L19 10l-5.1 1.9L12 17l-1.9-5.1L5 10l5.1-1.9L12 3z"
                      stroke-linecap="round"
                      stroke-linejoin="round"
                    />
                    <path d="M19 15l.9 2.1L22 18l-2.1.9L19 21l-.9-2.1L16 18l2.1-.9L19 15z" />
                  </svg>
                  <div class="min-w-0">
                    <h2
                      id="whats-new-title"
                      class="text-lg font-semibold text-base-content truncate"
                    >
                      Pulse v{version()} changelog
                    </h2>
                    <p class="text-xs text-muted">What changed in this release</p>
                  </div>
                </div>
                <ActionIconButton
                  onClick={dismissChangelog}
                  label="Dismiss what's new"
                  title="Close"
                  tone="muted"
                  size="md"
                  type="button"
                >
                  <XIcon class="h-5 w-5" aria-hidden="true" />
                </ActionIconButton>
              </div>
            </div>

            <div
              class="px-6 py-4 max-h-[60vh] overflow-y-auto text-sm text-base-content [&_h3]:mt-5 [&_h3:first-child]:mt-0 [&_h3]:mb-2 [&_h3]:text-base [&_h3]:font-semibold [&_ul]:list-disc [&_ul]:pl-5 [&_ul]:space-y-2 [&_ol]:list-decimal [&_ol]:pl-5 [&_ol]:space-y-2 [&_p]:mt-2 [&_a]:underline [&_code]:font-mono [&_code]:text-xs"
              // eslint-disable-next-line solid/no-innerhtml -- renderMarkdown sanitizes via DOMPurify
              innerHTML={changelogHtml()}
            />

            <div class="px-6 py-4 bg-surface-alt border-t border-border flex items-center justify-between gap-3">
              <ExternalTextLink
                href={buildReleaseNotesUrl(version())}
                variant="inline"
                class="text-sm"
              >
                Full release notes →
              </ExternalTextLink>
              <Button onClick={dismissChangelog} variant="primary" size="md" type="button">
                Got it
              </Button>
            </div>
          </div>
        </Dialog>
      </Show>
    </>
  );
}
