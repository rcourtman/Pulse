import { Match, Show, Switch, createEffect, createSignal, onCleanup } from 'solid-js';
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
import { formatUpdateReleaseDate } from '@/utils/updatesPresentation';
import {
  extractWhatsNew,
  isReleaseVersion,
  type WhatsNewContent as ReleaseContent,
} from '@/components/whatsNewModel';
import { WhatsNewContent } from '@/components/WhatsNewContent';
import { logger } from '@/utils/logger';

type WhatsNewLoadState = 'idle' | 'loading' | 'ready' | 'empty' | 'error';

type WhatsNewOpener = () => void;

let activeWhatsNewOpener: WhatsNewOpener | null = null;

/**
 * Open the running release's What's New dialog on demand (for example, from
 * the footer version or Settings -> Updates). Returns true when a mounted
 * dialog host handled the request.
 */
export function openWhatsNewDialog(): boolean {
  if (!activeWhatsNewOpener) return false;
  activeWhatsNewOpener();
  return true;
}

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
 * Post-update "What's New" notice and on-demand release dialog. A compact
 * non-blocking notice appears once after the running version changes (when
 * `allowAutoNotice` is enabled) and only when that release has user-facing
 * notes. The detailed dialog opens only after an explicit action from the
 * notice, the footer version, or Settings -> Updates.
 *
 * Telemetry payload changes are not announced here. They are disclosed in
 * release notes and the dated "Payload changes" section of docs/PRIVACY.md;
 * the Settings payload preview always shows the exact current contract.
 */
export function WhatsNewCard(props: { allowAutoNotice?: boolean } = {}) {
  const [noticeVisible, setNoticeVisible] = createSignal(false);
  const [dialogVisible, setDialogVisible] = createSignal(false);
  const [version, setVersion] = createSignal('');
  const [releaseDate, setReleaseDate] = createSignal<string | undefined>();
  const [content, setContent] = createSignal<ReleaseContent | null>(null);
  const [loadState, setLoadState] = createSignal<WhatsNewLoadState>('idle');
  let checked = false;
  let requestSequence = 0;
  let triggerElement: HTMLElement | null = null;

  const getEligibleVersion = (): string | null => {
    const info = updateStore.versionInfo();
    if (!info || info.isDevelopment || info.isSourceBuild || !isReleaseVersion(info.version)) {
      return null;
    }
    const normalized = normalizeReleaseVersion(info.version);
    return normalized || null;
  };

  const fetchNotes = async (
    currentVersion: string,
    mode: 'auto' | 'manual',
    noticeSlotReserved = false,
  ) => {
    const seq = ++requestSequence;
    if (mode === 'manual') {
      setLoadState('loading');
    }
    try {
      const notes = await UpdatesAPI.getReleaseNotes();
      if (seq !== requestSequence) return;
      // Mid-update the backend can briefly disagree with the UI about the
      // running version; don't show notes for the wrong release.
      if (normalizeReleaseVersion(notes.version) !== currentVersion) {
        if (mode === 'manual') {
          setLoadState('empty');
        }
        return;
      }
      const releaseContent = extractWhatsNew(notes.releaseNotes, currentVersion);
      if (!releaseContent) {
        markVersionSeen(currentVersion);
        if (mode === 'manual') {
          setContent(null);
          setLoadState('empty');
        }
        return;
      }
      setContent(releaseContent);
      setVersion(currentVersion);
      setReleaseDate(formatUpdateReleaseDate(notes.releaseDate));
      setLoadState('ready');
      // The compact notice is the only automatic release UI. Record the
      // version immediately so reloads cannot turn it into a recurring prompt.
      markVersionSeen(currentVersion);
      if (mode === 'auto' && noticeSlotReserved) {
        setNoticeVisible(true);
      }
    } catch (error) {
      if (seq !== requestSequence) return;
      if ((error as { status?: number }).status === 404) {
        // No published release for this build — stop asking automatically.
        markVersionSeen(currentVersion);
        if (mode === 'manual') {
          setContent(null);
          setLoadState('empty');
        }
        return;
      }
      // Transient failure: leave last-seen untouched so the next load retries.
      logger.warn("Failed to load release notes for What's New dialog", error);
      if (mode === 'manual') {
        setLoadState('error');
      }
    }
  };

  const openOnDemand = () => {
    const currentVersion = getEligibleVersion();
    if (!currentVersion) return;

    const alreadyLoaded = Boolean(
      content() && version() === currentVersion && loadState() === 'ready',
    );
    triggerElement = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    setNoticeVisible(false);
    setVersion(currentVersion);
    setDialogVisible(true);

    if (alreadyLoaded) {
      return;
    }
    void fetchNotes(currentVersion, 'manual');
  };

  activeWhatsNewOpener = openOnDemand;
  onCleanup(() => {
    if (activeWhatsNewOpener === openOnDemand) {
      activeWhatsNewOpener = null;
    }
  });

  createEffect(() => {
    if (props.allowAutoNotice === false) return;
    const info = updateStore.versionInfo();
    if (!info || checked) return;
    checked = true;

    const currentVersion = getEligibleVersion();
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
    void fetchNotes(currentVersion, 'auto', noticeSlotReserved);
  });

  const dismissNotice = () => {
    setNoticeVisible(false);
  };

  const openChangelog = () => {
    triggerElement = null;
    setNoticeVisible(false);
    setDialogVisible(true);
  };

  const retryLoad = () => {
    const currentVersion = getEligibleVersion() ?? version();
    if (!currentVersion) return;
    void fetchNotes(currentVersion, 'manual');
  };

  const dismissChangelog = () => {
    setDialogVisible(false);
  };

  return (
    <>
      <Show when={noticeVisible()}>
        <aside
          class="fixed bottom-(--pulse-mobile-nav-height) left-4 right-4 z-30 max-w-sm md:right-auto md:bottom-4"
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
          panelClass={content()?.visuals.length ? 'max-w-3xl' : 'max-w-2xl'}
          ariaLabelledBy="whats-new-title"
          returnFocus={() =>
            triggerElement?.isConnected ? triggerElement : document.getElementById('main')
          }
        >
          <div class="w-full" data-testid="whats-new-modal">
            <div class="px-6 py-4 border-b border-border">
              <div class="flex items-center justify-between gap-3">
                <div class="flex items-center gap-3 min-w-0">
                  {/* Sparkle icon */}
                  <svg
                    class="w-5 h-5 shrink-0 text-emerald-600 dark:text-emerald-400"
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
                      class="text-base font-semibold text-base-content sm:text-lg"
                    >
                      What's new in Pulse <span class="whitespace-nowrap">v{version()}</span>
                    </h2>
                    <Show when={releaseDate()}>
                      {(date) => <p class="text-xs text-muted">Released {date()}</p>}
                    </Show>
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

            <div class="max-h-[70vh] overflow-y-auto overscroll-contain px-4 py-5 sm:px-6">
              <Switch>
                <Match when={loadState() === 'loading'}>
                  <p class="py-6 text-center text-sm text-muted" role="status">
                    Loading release notes…
                  </p>
                </Match>
                <Match when={loadState() === 'error'}>
                  <div class="py-6 text-center space-y-3" role="alert">
                    <p class="text-sm text-muted">
                      Couldn't load release notes for v{version()} right now.
                    </p>
                    <Button onClick={retryLoad} variant="secondary" size="sm" type="button">
                      Try again
                    </Button>
                  </div>
                </Match>
                <Match when={loadState() === 'empty'}>
                  <p class="py-6 text-center text-sm text-muted">
                    No release notes are published for v{version()} yet.
                  </p>
                </Match>
                <Match when={content()}>
                  {(releaseContent) => <WhatsNewContent content={releaseContent()} />}
                </Match>
              </Switch>
            </div>

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
