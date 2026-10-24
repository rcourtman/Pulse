import { For, Show, createSignal } from 'solid-js';
import DOMPurify from 'dompurify';
import { marked } from 'marked';
import { ExternalTextLink } from './shared/ExternalTextLink';
import type { WhatsNewContent as ReleaseContent } from './whatsNewModel';

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
      class="text-sm leading-relaxed text-base-content [&_h3]:mb-3 [&_h3]:mt-8 [&_h3:first-child]:mt-0 [&_h3]:text-base [&_h3]:font-semibold [&_h4]:mt-3 [&_h4]:font-medium [&_ul]:list-disc [&_ul]:space-y-2 [&_ul]:pl-5 [&_ol]:list-decimal [&_ol]:pl-5 [&_p+p]:mt-2 [&_a]:text-blue-600 [&_a]:underline dark:[&_a]:text-blue-400 [&_code]:font-mono [&_code]:text-xs"
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

// A conventional changelog: warnings first, then every change group as plain
// headed lists, with nothing hidden behind a disclosure.
export function WhatsNewContent(props: { content: ReleaseContent }) {
  return (
    <div class="space-y-6">
      <Show when={props.content.important}>
        {(important) => (
          <section
            aria-label="Important release information"
            class="rounded-lg border border-amber-500/40 bg-amber-500/5 p-4"
          >
            <ReleaseText markdown={important()} />
          </section>
        )}
      </Show>
      <For each={props.content.visuals}>
        {(visual) => (
          <section aria-label={visual.title} class="space-y-3">
            <div>
              <h3 class="font-semibold text-base-content">{visual.title}</h3>
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
      <Show when={props.content.changes}>
        {(changes) => (
          <section aria-label="Changes">
            <ReleaseText markdown={changes()} />
          </section>
        )}
      </Show>
      <Show when={props.content.upgradeNotes}>
        {(upgradeNotes) => (
          <section aria-label="Upgrade notes">
            <ReleaseText markdown={`### Upgrade notes\n\n${upgradeNotes()}`} />
          </section>
        )}
      </Show>
    </div>
  );
}
