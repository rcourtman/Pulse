import { useLocation, useNavigate, useParams } from '@solidjs/router';
import { Show, createEffect, createMemo, createResource, onCleanup } from 'solid-js';
import { Button } from '@/components/shared/Button';
import { PageHeader } from '@/components/shared/PageHeader';
import {
  docAssetUrlForPath,
  docRouteForPath,
  extractDocTitle,
  normalizeDocPath,
  renderDocMarkdown,
  scrollToDocFragment,
  stripLeadingTitle,
} from '@/features/docs/docMarkdown';

const DOCS_INDEX_PATH = 'README';

const LOAD_FAILURE_MESSAGES = {
  missing: "That document is not part of this installation's shipped documentation set.",
  access:
    'Access to this guide was refused. Check that you are still signed in and have access, then try again.',
  unavailable:
    'This guide could not be loaded. There may be a temporary connection or server problem. Try again.',
};

class DocLoadError extends Error {
  constructor(
    readonly kind: keyof typeof LOAD_FAILURE_MESSAGES,
    readonly docPath: string,
  ) {
    // Keep response bodies and transport details out of the reading surface.
    super(LOAD_FAILURE_MESSAGES[kind]);
  }
}

async function fetchDoc(docPath: string): Promise<string> {
  try {
    const response = await fetch(docAssetUrlForPath(docPath), {
      headers: { Accept: 'text/markdown, text/plain' },
    });
    if (!response.ok) {
      const kind =
        response.status === 404 || response.status === 410
          ? 'missing'
          : response.status === 401 || response.status === 403
            ? 'access'
            : 'unavailable';
      throw new DocLoadError(kind, docPath);
    }
    // Both static handlers can return the SPA's HTML for a missing .md asset.
    // Check status first: an HTML access/error page is not evidence of absence.
    if (isHtmlResponse(response)) {
      throw new DocLoadError('missing', docPath);
    }
    return await response.text();
  } catch (error) {
    if (error instanceof DocLoadError) throw error;
    // A connection failure or interrupted response body can both be retried.
    throw new DocLoadError('unavailable', docPath);
  }
}

export function isHtmlResponse(response: Pick<Response, 'headers'>): boolean {
  const contentType = response.headers.get('content-type') ?? '';
  return contentType.toLowerCase().includes('text/html');
}

export default function Docs() {
  const params = useParams<{ docPath?: string }>();
  const navigate = useNavigate();
  const location = useLocation();
  let article: HTMLElement | undefined;

  const docPath = createMemo(() => normalizeDocPath(params.docPath ?? '') || DOCS_INDEX_PATH);
  const [document, { refetch }] = createResource(docPath, fetchDoc);

  // Keep a failed guide's retry control mounted while that same read is in
  // flight, but never present its error as a different guide loads.
  const loadError = createMemo(() => {
    const error: unknown = document.error;
    return error instanceof DocLoadError && error.docPath === docPath() ? error : undefined;
  });

  // Reading a resource that rejected re-throws, so the error has to be checked
  // before the accessor is touched or the failure escapes the memo instead of
  // reaching the error branch below.
  const markdown = createMemo(() => (document.error ? undefined : document()));

  const title = createMemo(() => {
    const fallback = docPath().split('/').pop() ?? 'Documentation';
    const source = markdown();
    return source ? extractDocTitle(source, fallback) : fallback;
  });

  const html = createMemo(() => {
    const source = markdown();
    // The leading heading is promoted into the page header above.
    return source ? renderDocMarkdown(stripLeadingTitle(source), docPath()) : '';
  });

  // A direct link or cross-document navigation can arrive before fetchDoc
  // finishes. Retry once the rendered content exists, without timers or
  // observers that could later pull the reader away from their position.
  createEffect(() => {
    const content = html();
    const hash = location.hash;
    if (!content || !hash) return;
    const frame = requestAnimationFrame(() => {
      if (article) scrollToDocFragment(article, hash);
    });
    onCleanup(() => cancelAnimationFrame(frame));
  });

  // Intra-documentation links are rewritten to viewer routes by the renderer.
  // Delegate their clicks to the router so following one does not reload the
  // application shell. Re-activating the current fragment is still a request
  // to return to it, but changes no reactive URL state for the effect above.
  const handleClick = (event: MouseEvent) => {
    if (event.defaultPrevented || event.button !== 0) return;
    if (event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
    const anchor = (event.target as Element | null)?.closest?.<HTMLAnchorElement>(
      'a[data-doc-link]',
    );
    const href = anchor?.getAttribute('href');
    if (!href || anchor?.hasAttribute('download') || (anchor?.target && anchor.target !== '_self'))
      return;
    let destination: URL;
    try {
      destination = new URL(href, window.location.href);
    } catch {
      return;
    }
    if (destination.origin !== window.location.origin) return;
    event.preventDefault();
    if (
      destination.pathname === location.pathname &&
      destination.search === location.search &&
      destination.hash === location.hash &&
      destination.hash
    ) {
      if (article) scrollToDocFragment(article, destination.hash);
      return;
    }
    navigate(`${destination.pathname}${destination.search}${destination.hash}`);
  };

  return (
    <div class="mx-auto w-full max-w-4xl space-y-4">
      <PageHeader
        title={title()}
        description="Documentation shipped with this Pulse installation."
      />

      <Show when={docPath() !== DOCS_INDEX_PATH}>
        <a
          href={docRouteForPath(DOCS_INDEX_PATH)}
          data-doc-link=""
          onClick={handleClick}
          class="inline-flex items-center gap-1 text-sm text-blue-600 hover:underline dark:text-blue-400"
        >
          &larr; All documentation
        </a>
      </Show>

      <Show
        when={!document.loading || loadError()}
        fallback={
          <p role="status" class="text-sm text-muted">
            Loading documentation&hellip;
          </p>
        }
      >
        <Show
          when={!loadError()}
          fallback={
            <div class="space-y-2">
              <p role="alert" class="text-sm text-muted">
                {loadError()?.message}
              </p>
              <Button
                size="sm"
                // Native disabled blurs the keyboard reader's control. Keep it
                // focusable, announce its state and reject duplicate reads here.
                aria-disabled={document.loading}
                class="aria-disabled:cursor-wait aria-disabled:opacity-50"
                onClick={() => {
                  if (!document.loading) void refetch();
                }}
              >
                Try again
              </Button>
              <Show when={document.loading}>
                <p role="status" class="text-sm text-muted">
                  Loading documentation&hellip;
                </p>
              </Show>
              <a
                href={docRouteForPath(DOCS_INDEX_PATH)}
                data-doc-link=""
                onClick={handleClick}
                class="block text-sm text-blue-600 hover:underline dark:text-blue-400"
              >
                Back to the documentation index
              </a>
            </div>
          }
        >
          <article
            ref={article}
            // Typography renders inline code wrapped in literal backticks by
            // default, which reads as unrendered markdown. Tables get their
            // scroll container from wrapTables rather than prose utilities.
            // Inline code must be able to break, because a long unbreakable
            // URL in one otherwise pushes the whole page into horizontal
            // scrolling on a phone; code inside pre keeps its own scrollbar.
            class="prose prose-sm max-w-none dark:prose-invert prose-headings:scroll-mt-20 prose-pre:overflow-x-auto prose-code:before:content-none prose-code:after:content-none [&_:not(pre)>code]:wrap-break-word"
            onClick={handleClick}
            // eslint-disable-next-line solid/no-innerhtml -- renderDocMarkdown sanitises with DOMPurify
            innerHTML={html()}
          />
        </Show>
      </Show>
    </div>
  );
}
