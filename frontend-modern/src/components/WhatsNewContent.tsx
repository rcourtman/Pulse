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
      class="text-sm leading-relaxed text-base-content [&_h3]:mb-2 [&_h3]:mt-5 [&_h3:first-child]:mt-0 [&_h3]:font-semibold [&_h4]:mt-3 [&_h4]:font-medium [&_ul]:list-disc [&_ul]:space-y-2 [&_ul]:pl-5 [&_ol]:list-decimal [&_ol]:pl-5 [&_p+p]:mt-2 [&_a]:text-blue-600 [&_a]:underline dark:[&_a]:text-blue-400 [&_code]:font-mono [&_code]:text-xs"
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

export function WhatsNewContent(props: { content: ReleaseContent }) {
  return (
    <div class="space-y-6">
      <Show when={props.content.highlights.length > 0}>
        <section aria-label="Release highlights">
          <h3 class="mb-3 text-xs font-semibold uppercase tracking-wide text-muted">Highlights</h3>
          <ul class="space-y-3">
            <For each={props.content.highlights}>
              {(highlight) => (
                <li class="rounded-lg border border-border bg-surface-alt p-4">
                  <ReleaseText markdown={highlight} />
                </li>
              )}
            </For>
          </ul>
        </section>
      </Show>
      <Show when={props.content.important}>
        {(important) => (
          <section
            aria-label="Important release information"
            class="rounded-lg border border-border p-4"
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
          <details
            open={props.content.highlights.length === 0 && props.content.visuals.length === 0}
            class="group rounded-lg border border-border"
          >
            <summary class="cursor-pointer rounded-lg px-4 py-3 text-sm font-medium text-base-content focus-visible:outline-solid focus-visible:outline-2 focus-visible:outline-blue-500">
              All changes
            </summary>
            <div class="border-t border-border p-4">
              <ReleaseText markdown={changes()} />
            </div>
          </details>
        )}
      </Show>
    </div>
  );
}
