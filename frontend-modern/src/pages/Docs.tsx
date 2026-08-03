import { useNavigate, useParams } from '@solidjs/router';
import { Show, createMemo, createResource } from 'solid-js';
import { PageHeader } from '@/components/shared/PageHeader';
import {
  docAssetUrlForPath,
  docRouteForPath,
  extractDocTitle,
  normalizeDocPath,
  renderDocMarkdown,
  stripLeadingTitle,
} from '@/features/docs/docMarkdown';

const DOCS_INDEX_PATH = 'README';

async function fetchDoc(docPath: string): Promise<string> {
  const response = await fetch(docAssetUrlForPath(docPath), {
    headers: { Accept: 'text/markdown, text/plain' },
  });
  if (!response.ok) {
    throw new Error(String(response.status));
  }
  // A document that is not shipped does not 404. Both the Go static handler
  // and the dev server fall back to index.html for any unmatched non-API
  // path, including one ending in .md, so the only reliable signal that the
  // asset is missing is getting HTML back instead of markdown.
  if (isHtmlResponse(response)) {
    throw new Error('not-shipped');
  }
  return response.text();
}

export function isHtmlResponse(response: Pick<Response, 'headers'>): boolean {
  const contentType = response.headers.get('content-type') ?? '';
  return contentType.toLowerCase().includes('text/html');
}

export default function Docs() {
  const params = useParams<{ docPath?: string }>();
  const navigate = useNavigate();

  const docPath = createMemo(() => normalizeDocPath(params.docPath ?? '') || DOCS_INDEX_PATH);
  const [document] = createResource(docPath, fetchDoc);

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

  // Intra-documentation links are rewritten to viewer routes by the renderer.
  // Delegate their clicks to the router so following one does not reload the
  // application shell.
  const handleClick = (event: MouseEvent) => {
    if (event.defaultPrevented || event.button !== 0) return;
    if (event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
    const anchor = (event.target as Element | null)?.closest?.('a[data-doc-link]');
    const href = anchor?.getAttribute('href');
    if (!href) return;
    event.preventDefault();
    navigate(href);
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
        when={!document.loading}
        fallback={<p class="text-sm text-muted">Loading documentation&hellip;</p>}
      >
        <Show
          when={!document.error}
          fallback={
            <div class="space-y-2">
              <p class="text-sm text-muted">
                That document is not part of this installation's shipped documentation set.
              </p>
              <a
                href={docRouteForPath(DOCS_INDEX_PATH)}
                data-doc-link=""
                onClick={handleClick}
                class="text-sm text-blue-600 hover:underline dark:text-blue-400"
              >
                Back to the documentation index
              </a>
            </div>
          }
        >
          <article
            // Typography renders inline code wrapped in literal backticks by
            // default, which reads as unrendered markdown. Tables get their
            // scroll container from wrapTables rather than prose utilities.
            // Inline code must be able to break, because a long unbreakable
            // URL in one otherwise pushes the whole page into horizontal
            // scrolling on a phone; code inside pre keeps its own scrollbar.
            class="prose prose-sm max-w-none dark:prose-invert prose-pre:overflow-x-auto prose-code:before:content-none prose-code:after:content-none [&_:not(pre)>code]:break-words"
            onClick={handleClick}
            // eslint-disable-next-line solid/no-innerhtml -- renderDocMarkdown sanitises with DOMPurify
            innerHTML={html()}
          />
        </Show>
      </Show>
    </div>
  );
}
