import { Show, createEffect, createSignal } from 'solid-js';
import XIcon from 'lucide-solid/icons/x';
import { updateStore } from '@/stores/updates';
import { UpdatesAPI } from '@/api/updates';
import { STORAGE_KEYS } from '@/utils/localStorage';
import { ActionIconButton, Button } from '@/components/shared/Button';
import { Dialog } from '@/components/shared/Dialog';
import { ExternalTextLink } from '@/components/shared/ExternalTextLink';
import { buildReleaseNotesUrl, normalizeReleaseVersion } from '@/components/updateVersion';
import { extractHighlights, isReleaseVersion } from '@/components/whatsNewModel';
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
 * Post-update "What's New" dialog. Shows once after the running version
 * changes, and only when that release has a curated `## Highlights` section
 * in its GitHub release notes. Dismissing (or a highlights-free release)
 * records the version so the dialog stays quiet until the next update.
 */
export function WhatsNewCard() {
  const [visible, setVisible] = createSignal(false);
  const [version, setVersion] = createSignal('');
  const [highlightsHtml, setHighlightsHtml] = createSignal('');
  let checked = false;

  const loadNotes = async (currentVersion: string) => {
    try {
      const notes = await UpdatesAPI.getReleaseNotes();
      // Mid-update the backend can briefly disagree with the UI about the
      // running version; don't show notes for the wrong release.
      if (normalizeReleaseVersion(notes.version) !== currentVersion) {
        return;
      }
      const highlights = extractHighlights(notes.releaseNotes);
      if (!highlights) {
        markVersionSeen(currentVersion);
        return;
      }
      setHighlightsHtml(renderMarkdown(highlights));
      setVersion(currentVersion);
      setVisible(true);
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

    void loadNotes(currentVersion);
  });

  // Any close path (button, backdrop, Escape) counts as seen.
  const dismiss = () => {
    markVersionSeen(version());
    setVisible(false);
  };

  return (
    <Show when={visible()}>
      <Dialog
        isOpen={visible()}
        onClose={dismiss}
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
                  <h2 id="whats-new-title" class="text-lg font-semibold text-base-content truncate">
                    What's new in v{version()}
                  </h2>
                  <p class="text-xs text-muted">Pulse updated successfully</p>
                </div>
              </div>
              <ActionIconButton
                onClick={dismiss}
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
            class="px-6 py-4 max-h-[60vh] overflow-y-auto text-sm text-base-content [&_ul]:list-disc [&_ul]:pl-5 [&_ul]:space-y-2 [&_ol]:list-decimal [&_ol]:pl-5 [&_ol]:space-y-2 [&_p]:mt-2 [&_a]:underline [&_code]:font-mono [&_code]:text-xs"
            // eslint-disable-next-line solid/no-innerhtml -- renderMarkdown sanitizes via DOMPurify
            innerHTML={highlightsHtml()}
          />

          <div class="px-6 py-4 bg-surface-alt border-t border-border flex items-center justify-between gap-3">
            <ExternalTextLink
              href={buildReleaseNotesUrl(version())}
              variant="inline"
              class="text-sm"
            >
              Full release notes →
            </ExternalTextLink>
            <Button onClick={dismiss} variant="primary" size="md" type="button">
              Got it
            </Button>
          </div>
        </div>
      </Dialog>
    </Show>
  );
}
