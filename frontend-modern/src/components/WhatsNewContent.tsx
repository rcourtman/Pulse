import { For, Show, createMemo, createSignal } from 'solid-js';
import DOMPurify from 'dompurify';
import { marked } from 'marked';
import AlertTriangleIcon from 'lucide-solid/icons/alert-triangle';
import ChevronDownIcon from 'lucide-solid/icons/chevron-down';
import { ExternalTextLink } from './shared/ExternalTextLink';
import type { WhatsNewContent as ReleaseContent } from './whatsNewModel';

const COMPACT_KNOWN_ISSUES_MAX_ITEMS = 2;
const COMPACT_KNOWN_ISSUES_MAX_CHARS = 420;
const COMPACT_CHANGE_GROUPS_MAX_SECTIONS = 2;
const COMPACT_CHANGE_GROUPS_MAX_ITEMS = 12;
const COMPACT_CHANGE_GROUPS_MAX_CHARS = 1600;
const COMPACT_UPGRADE_NOTES_MAX_ITEMS = 1;
const COMPACT_UPGRADE_NOTES_MAX_CHARS = 320;

// Release text and screenshot assets have separate rendering boundaries. Images
// never enter this HTML renderer, and text cannot supply application classes.
const releaseText = (markdown: string): string =>
  DOMPurify.sanitize(marked.parse(markdown, { async: false }), {
    ALLOWED_TAGS: [
      'p',
      'br',
      'strong',
      'em',
      'code',
      'pre',
      'blockquote',
      'ul',
      'ol',
      'li',
      'h3',
      'h4',
      'a',
    ],
    ALLOWED_ATTR: ['href'],
    ALLOWED_URI_REGEXP: /^(?:https?:|mailto:)/i,
  });

function ReleaseText(props: { markdown: string }) {
  return (
    <div
      class="text-sm leading-relaxed text-base-content/85 [&_strong]:font-semibold [&_strong]:text-base-content [&_h3]:mb-2.5 [&_h3]:mt-6 [&_h3:first-child]:mt-0 [&_h3]:text-sm [&_h3]:font-semibold [&_h3]:text-base-content [&_h4]:mt-3 [&_h4]:font-medium [&_h4]:text-base-content [&_ul]:list-disc [&_ul]:space-y-2.5 [&_ul]:pl-5 [&_ol]:list-decimal [&_ol]:space-y-2 [&_ol]:pl-5 [&_p+p]:mt-2 [&_a]:text-blue-600 [&_a]:underline dark:[&_a]:text-blue-400 [&_code]:font-mono [&_code]:text-xs"
      // eslint-disable-next-line solid/no-innerhtml -- releaseText sanitizes release Markdown
      innerHTML={releaseText(props.markdown)}
    />
  );
}

function Screenshot(props: { src: string; title: string; label: string }) {
  const [failed, setFailed] = createSignal(false);
  return (
    <figure class="min-w-0 overflow-hidden rounded-lg border border-border bg-surface">
      <figcaption class="border-b border-border px-3 py-2 text-xs font-medium text-muted">
        {props.label}
      </figcaption>
      <Show
        when={!failed()}
        fallback={<p class="p-4 text-sm text-muted">Screenshot unavailable.</p>}
      >
        <ExternalTextLink
          href={props.src}
          variant="inline"
          class="block"
          aria-label={`View ${props.title}, ${props.label.toLowerCase()}, full size`}
        >
          <img
            src={props.src.replace(
              'https://github.com/rcourtman/Pulse/releases/download/',
              '/api/updates/release-notes/assets/',
            )}
            alt={`${props.title}, ${props.label.toLowerCase()}`}
            class="h-auto w-full"
            loading="lazy"
            decoding="async"
            referrerPolicy="no-referrer"
            onError={() => setFailed(true)}
          />
        </ExternalTextLink>
      </Show>
    </figure>
  );
}

/**
 * Post-update release view designed for quick scanning: Breaking changes always
 * lead, followed by Highlights and optional screenshots. Short Known issues,
 * compact change lists, and brief Upgrade notes render inline, while long
 * lists collapse into expandable sections with item counts so large releases
 * stay approachable.
 */
export function WhatsNewContent(props: { content: ReleaseContent }) {
  const [knownIssuesOpen, setKnownIssuesOpen] = createSignal(false);
  const [upgradeNotesOpen, setUpgradeNotesOpen] = createSignal(false);
  const [expandedGroups, setExpandedGroups] = createSignal<Record<string, boolean>>({});

  const knownIssuesCompact = createMemo(
    () =>
      props.content.knownIssuesCount <= COMPACT_KNOWN_ISSUES_MAX_ITEMS &&
      (props.content.knownIssues?.length ?? 0) <= COMPACT_KNOWN_ISSUES_MAX_CHARS,
  );

  const upgradeNotesCompact = createMemo(
    () =>
      props.content.upgradeNotesCount <= COMPACT_UPGRADE_NOTES_MAX_ITEMS &&
      (props.content.upgradeNotes?.length ?? 0) <= COMPACT_UPGRADE_NOTES_MAX_CHARS,
  );

  const totalChangeItems = createMemo(() =>
    props.content.changeGroups.reduce((sum, group) => sum + Math.max(group.itemCount, 1), 0),
  );

  const totalChangeChars = createMemo(() =>
    props.content.changeGroups.reduce((sum, group) => sum + group.body.length, 0),
  );

  const shouldCollapseGroups = createMemo(
    () =>
      props.content.changeGroups.length > COMPACT_CHANGE_GROUPS_MAX_SECTIONS ||
      totalChangeItems() > COMPACT_CHANGE_GROUPS_MAX_ITEMS ||
      totalChangeChars() > COMPACT_CHANGE_GROUPS_MAX_CHARS,
  );

  const isGroupExpanded = (label: string) => Boolean(expandedGroups()[label]);

  const allGroupsExpanded = createMemo(
    () =>
      props.content.changeGroups.length > 0 &&
      props.content.changeGroups.every((group) => Boolean(expandedGroups()[group.label])),
  );

  const toggleGroup = (label: string) => {
    setExpandedGroups((current) => ({ ...current, [label]: !current[label] }));
  };

  const toggleAllGroups = () => {
    const nextOpen = !allGroupsExpanded();
    const next: Record<string, boolean> = {};
    for (const group of props.content.changeGroups) {
      next[group.label] = nextOpen;
    }
    setExpandedGroups(next);
  };

  return (
    <div class="space-y-5">
      <Show when={props.content.breakingChanges}>
        {(breaking) => (
          <section
            aria-label="Breaking changes"
            class="rounded-lg border border-amber-500/40 bg-amber-500/5 p-4"
          >
            <div class="mb-2 flex items-center gap-2 text-amber-700 dark:text-amber-300">
              <AlertTriangleIcon class="h-4 w-4 shrink-0" aria-hidden="true" />
              <h3 class="text-sm font-semibold">Breaking changes</h3>
            </div>
            <ReleaseText markdown={breaking()} />
          </section>
        )}
      </Show>

      <Show when={props.content.highlights.length > 0}>
        <section aria-label="Highlights" class="space-y-2.5">
          <Show when={props.content.changeGroups.length > 0}>
            <h3 class="text-xs font-semibold uppercase tracking-wide text-muted">Highlights</h3>
          </Show>
          <ul class="space-y-2">
            <For each={props.content.highlights}>
              {(item) => (
                <li class="flex items-start gap-2.5 text-sm leading-relaxed text-base-content">
                  <span
                    class="mt-2 h-1.5 w-1.5 shrink-0 rounded-full bg-emerald-500 dark:bg-emerald-400"
                    aria-hidden="true"
                  />
                  <span>{item}</span>
                </li>
              )}
            </For>
          </ul>
        </section>
      </Show>

      <For each={props.content.visuals}>
        {(visual) => (
          <section aria-label={visual.title} class="space-y-3">
            <div>
              <h3 class="text-sm font-semibold text-base-content">{visual.title}</h3>
              <Show when={visual.description}>
                <div class="mt-1">
                  <ReleaseText markdown={visual.description} />
                </div>
              </Show>
            </div>
            <div class={visual.before ? 'grid gap-3 sm:grid-cols-2' : 'grid gap-3'}>
              <Show when={visual.before}>
                {(before) => <Screenshot src={before()} title={visual.title} label="Before" />}
              </Show>
              <Screenshot src={visual.after} title={visual.title} label="Now" />
            </div>
          </section>
        )}
      </For>

      <Show when={props.content.knownIssues}>
        {(knownIssues) => (
          <Show
            when={knownIssuesCompact()}
            fallback={
              <section
                aria-label="Known issues"
                class="overflow-hidden rounded-lg border border-amber-500/40 bg-amber-500/5"
              >
                <button
                  type="button"
                  aria-expanded={knownIssuesOpen()}
                  aria-controls="whats-new-known-issues"
                  onClick={() => setKnownIssuesOpen((open) => !open)}
                  class="flex w-full items-center justify-between gap-3 px-4 py-2.5 text-left transition-colors hover:bg-amber-500/10 focus-visible:outline-2 focus-visible:outline-offset-[-2px] focus-visible:outline-blue-500"
                >
                  <span class="flex min-w-0 items-center gap-2 text-amber-700 dark:text-amber-300">
                    <AlertTriangleIcon class="h-4 w-4 shrink-0" aria-hidden="true" />
                    <span class="text-sm font-medium">Known issues</span>
                  </span>
                  <span class="flex shrink-0 items-center gap-2">
                    <span class="rounded-full bg-amber-500/15 px-2 py-0.5 text-xs font-medium tabular-nums text-amber-800 dark:text-amber-200">
                      {props.content.knownIssuesCount}
                    </span>
                    <ChevronDownIcon
                      class={`h-4 w-4 text-amber-700 transition-transform dark:text-amber-300 ${
                        knownIssuesOpen() ? 'rotate-180' : ''
                      }`}
                      aria-hidden="true"
                    />
                  </span>
                </button>
                <Show when={knownIssuesOpen()}>
                  <div id="whats-new-known-issues" class="border-t border-amber-500/20 px-4 py-3">
                    <ReleaseText markdown={knownIssues()} />
                  </div>
                </Show>
              </section>
            }
          >
            <section
              aria-label="Known issues"
              class="rounded-lg border border-amber-500/40 bg-amber-500/5 p-3.5"
            >
              <div class="mb-2 flex items-center gap-2 text-amber-700 dark:text-amber-300">
                <AlertTriangleIcon class="h-4 w-4 shrink-0" aria-hidden="true" />
                <h3 class="text-sm font-semibold">Known issues</h3>
              </div>
              <ReleaseText markdown={knownIssues()} />
            </section>
          </Show>
        )}
      </Show>

      <Show when={props.content.changeGroups.length > 0}>
        <Show
          when={shouldCollapseGroups()}
          fallback={
            <section aria-label="Changes" class="space-y-4">
              <For each={props.content.changeGroups}>
                {(group) => (
                  <div class="space-y-2">
                    <h3 class="text-sm font-semibold text-base-content">{group.label}</h3>
                    <ReleaseText markdown={group.body} />
                  </div>
                )}
              </For>
            </section>
          }
        >
          <section aria-label="Changes" class="space-y-2">
            <div class="flex items-center justify-between gap-2">
              <h3 class="text-xs font-semibold uppercase tracking-wide text-muted">
                All changes ({totalChangeItems()})
              </h3>
              <button
                type="button"
                onClick={toggleAllGroups}
                class="rounded text-xs font-medium text-blue-600 hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-500 dark:text-blue-400"
              >
                {allGroupsExpanded() ? 'Collapse all' : 'Expand all'}
              </button>
            </div>
            <div class="divide-y divide-border overflow-hidden rounded-lg border border-border bg-surface">
              <For each={props.content.changeGroups}>
                {(group, index) => {
                  const panelId = `whats-new-group-${index()}`;
                  return (
                    <div>
                      <button
                        type="button"
                        aria-expanded={isGroupExpanded(group.label)}
                        aria-controls={panelId}
                        onClick={() => toggleGroup(group.label)}
                        class="flex w-full items-center justify-between gap-3 px-4 py-2.5 text-left transition-colors hover:bg-surface-alt/60 focus-visible:outline-2 focus-visible:outline-offset-[-2px] focus-visible:outline-blue-500"
                      >
                        <span class="min-w-0 text-sm font-medium text-base-content">
                          {group.label}
                        </span>
                        <span class="flex shrink-0 items-center gap-2">
                          <span class="rounded-full bg-surface-alt px-2 py-0.5 text-xs tabular-nums text-muted">
                            {group.itemCount}
                          </span>
                          <ChevronDownIcon
                            class={`h-4 w-4 text-muted transition-transform ${
                              isGroupExpanded(group.label) ? 'rotate-180' : ''
                            }`}
                            aria-hidden="true"
                          />
                        </span>
                      </button>
                      <Show when={isGroupExpanded(group.label)}>
                        <div
                          id={panelId}
                          class="border-t border-border/60 bg-surface-alt/25 px-4 py-3"
                        >
                          <ReleaseText markdown={group.body} />
                        </div>
                      </Show>
                    </div>
                  );
                }}
              </For>
            </div>
          </section>
        </Show>
      </Show>

      <Show when={props.content.upgradeNotes}>
        {(upgradeNotes) => (
          <Show
            when={upgradeNotesCompact()}
            fallback={
              <section
                aria-label="Upgrade notes"
                class="overflow-hidden rounded-lg border border-border bg-surface"
              >
                <button
                  type="button"
                  aria-expanded={upgradeNotesOpen()}
                  aria-controls="whats-new-upgrade-notes"
                  onClick={() => setUpgradeNotesOpen((open) => !open)}
                  class="flex w-full items-center justify-between gap-3 px-4 py-2.5 text-left transition-colors hover:bg-surface-alt/60 focus-visible:outline-2 focus-visible:outline-offset-[-2px] focus-visible:outline-blue-500"
                >
                  <span class="text-sm font-medium text-base-content">Upgrade notes</span>
                  <span class="flex shrink-0 items-center gap-2">
                    <span class="rounded-full bg-surface-alt px-2 py-0.5 text-xs tabular-nums text-muted">
                      {props.content.upgradeNotesCount}
                    </span>
                    <ChevronDownIcon
                      class={`h-4 w-4 text-muted transition-transform ${
                        upgradeNotesOpen() ? 'rotate-180' : ''
                      }`}
                      aria-hidden="true"
                    />
                  </span>
                </button>
                <Show when={upgradeNotesOpen()}>
                  <div
                    id="whats-new-upgrade-notes"
                    class="border-t border-border/60 bg-surface-alt/25 px-4 py-3"
                  >
                    <ReleaseText markdown={upgradeNotes()} />
                  </div>
                </Show>
              </section>
            }
          >
            <section aria-label="Upgrade notes" class="space-y-2 border-t border-border pt-4">
              <h3 class="text-xs font-semibold uppercase tracking-wide text-muted">
                Upgrade notes
              </h3>
              <ReleaseText markdown={upgradeNotes()} />
            </section>
          </Show>
        )}
      </Show>
    </div>
  );
}
