import { useLocation, useNavigate, useParams } from '@solidjs/router';
import {
  For,
  Show,
  createEffect,
  createMemo,
  createResource,
  createSignal,
  onCleanup,
} from 'solid-js';
import ArrowLeft from 'lucide-solid/icons/arrow-left';
import ChevronDown from 'lucide-solid/icons/chevron-down';
import ChevronUp from 'lucide-solid/icons/chevron-up';
import List from 'lucide-solid/icons/list';
import { Button } from '@/components/shared/Button';
import { PageHeader } from '@/components/shared/PageHeader';
import { SearchInput } from '@/components/shared/SearchInput';
import {
  docAssetUrlForPath,
  docRouteForPath,
  extractDocTitle,
  normalizeDocPath,
  renderDocMarkdown,
  scrollToDocFragment,
  stripLeadingTitle,
} from '@/features/docs/docMarkdown';
import { getFeatureGuideAvailability } from '@/utils/featureReleases';

const DOCS_INDEX_PATH = 'README';

export interface DocTocItem {
  id: string;
  text: string;
  level: 2 | 3;
}

export interface FilteredDocsIndexResult {
  html: string;
  matchCount: number;
  totalCount: number;
}

export function extractDocToc(renderedHtml: string): DocTocItem[] {
  if (!renderedHtml || typeof globalThis.document === 'undefined') return [];
  const container = globalThis.document.createElement('div');
  container.innerHTML = renderedHtml;
  const items: DocTocItem[] = [];
  container.querySelectorAll('h2[id], h3[id]').forEach((heading) => {
    const id = heading.getAttribute('id')?.trim() ?? '';
    const text = heading.textContent?.trim() ?? '';
    if (!id || !text) return;
    items.push({
      id,
      text,
      level: heading.tagName === 'H3' ? 3 : 2,
    });
  });
  return items;
}

export function filterDocsIndexHtml(
  renderedHtml: string,
  rawQuery: string,
): FilteredDocsIndexResult {
  if (!renderedHtml || typeof globalThis.document === 'undefined') {
    return { html: renderedHtml, matchCount: 0, totalCount: 0 };
  }
  const container = globalThis.document.createElement('div');
  container.innerHTML = renderedHtml;
  const totalCount = container.querySelectorAll('li').length;
  const tokens = rawQuery.trim().toLowerCase().split(/\s+/).filter(Boolean);
  if (tokens.length === 0) {
    return { html: renderedHtml, matchCount: totalCount, totalCount };
  }

  const matchesTokens = (text: string): boolean => {
    const lower = text.toLowerCase();
    return tokens.every((token) => lower.includes(token));
  };

  interface IndexSection {
    heading: Element | null;
    headingText: string;
    nodes: Element[];
  }

  const sections: IndexSection[] = [];
  let current: IndexSection = { heading: null, headingText: '', nodes: [] };
  sections.push(current);

  for (const child of Array.from(container.children)) {
    if (child.tagName === 'H2' || child.tagName === 'H3') {
      current = {
        heading: child,
        headingText: child.textContent?.trim() ?? '',
        nodes: [],
      };
      sections.push(current);
    } else {
      current.nodes.push(child);
    }
  }

  let matchCount = 0;
  for (const section of sections) {
    let sectionHasContent = false;
    for (const node of section.nodes) {
      if (node.tagName === 'UL' || node.tagName === 'OL') {
        const listItems = Array.from(node.children).filter((child) => child.tagName === 'LI');
        let keptItems = 0;
        for (const item of listItems) {
          const hrefs = Array.from(item.querySelectorAll('a[href]'))
            .map((anchor) => anchor.getAttribute('href') ?? '')
            .join(' ');
          const haystack = `${section.headingText} ${item.textContent ?? ''} ${hrefs}`;
          if (matchesTokens(haystack)) {
            keptItems += 1;
            matchCount += 1;
          } else {
            item.remove();
          }
        }
        if (keptItems === 0) {
          node.remove();
        } else {
          sectionHasContent = true;
        }
        continue;
      }

      const hrefs = Array.from(node.querySelectorAll('a[href]'))
        .map((anchor) => anchor.getAttribute('href') ?? '')
        .join(' ');
      const haystack = `${section.headingText} ${node.textContent ?? ''} ${hrefs}`;
      if (node.querySelector('a[href]') && matchesTokens(haystack)) {
        sectionHasContent = true;
      } else {
        node.remove();
      }
    }

    if (section.heading && !sectionHasContent) {
      section.heading.remove();
    }
  }

  return {
    html: container.innerHTML,
    matchCount,
    totalCount,
  };
}

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
  const isIndexPage = createMemo(() => docPath() === DOCS_INDEX_PATH);
  const [indexQuery, setIndexQuery] = createSignal('');
  const [tocOpenDesktop, setTocOpenDesktop] = createSignal(true);
  const [tocOpenMobile, setTocOpenMobile] = createSignal(false);

  // Normalize direct /docs/<NAME>.md routes to the viewer route cleanly so
  // bookmarking or pasting a markdown path stays inside the rendered viewer.
  createEffect(() => {
    const pathname = location.pathname;
    if (!/\.md$/i.test(pathname)) return;
    const canonicalPath = docRouteForPath(docPath());
    const targetHref = `${canonicalPath}${location.search}${location.hash}`;
    const currentHref = `${pathname}${location.search}${location.hash}`;
    if (targetHref !== currentHref) {
      navigate(targetHref, { replace: true });
    }
  });

  createEffect(() => {
    docPath();
    setIndexQuery('');
    setTocOpenMobile(false);
  });
  // The guide of a feature this build does not show is not part of what it
  // ships. Until the build has said what it is, nothing is asked for and
  // nothing is claimed: the page waits, as it does for any guide loading.
  const request = createMemo(
    () => {
      const availability = getFeatureGuideAvailability(docPath());
      return availability === 'pending'
        ? null
        : { docPath: docPath(), available: availability === 'on' };
    },
    undefined,
    {
      equals: (a, b) =>
        a === b ||
        (a !== null && b !== null && a.docPath === b.docPath && a.available === b.available),
    },
  );
  const [document, { refetch }] = createResource(request, (asked) =>
    asked.available
      ? fetchDoc(asked.docPath)
      : Promise.reject(new DocLoadError('missing', asked.docPath)),
  );
  const waiting = () => request() === null || document.loading;

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

  const filteredIndex = createMemo(() =>
    isIndexPage()
      ? filterDocsIndexHtml(html(), indexQuery())
      : { html: html(), matchCount: 0, totalCount: 0 },
  );

  const displayedHtml = createMemo(() => (isIndexPage() ? filteredIndex().html : html()));
  const tocItems = createMemo(() => (isIndexPage() ? [] : extractDocToc(html())));
  const showToc = createMemo(() => !waiting() && !loadError() && tocItems().length >= 2);
  const hasIndexQuery = createMemo(() => indexQuery().trim().length > 0);
  const showIndexFilterEmpty = createMemo(
    () => isIndexPage() && hasIndexQuery() && filteredIndex().matchCount === 0,
  );

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

  const handleAppLinkClick = (event: MouseEvent) => {
    if (event.defaultPrevented || event.button !== 0) return;
    if (event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
    event.preventDefault();
    navigate('/');
  };

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
    // Re-activating the current fragment changes neither reactive dependency
    // above. Restore its focus as well as its scroll position explicitly.
    if (
      article &&
      location.hash &&
      href === `${location.pathname}${location.search}${location.hash}`
    ) {
      scrollToDocFragment(article, location.hash);
      return;
    }
    navigate(href);
  };

  const renderTocList = (onSelect?: () => void) => (
    <ul class="space-y-1 text-xs">
      <For each={tocItems()}>
        {(item) => (
          <li class={item.level === 3 ? 'pl-3' : undefined}>
            <a
              href={`${docRouteForPath(docPath())}#${item.id}`}
              data-doc-link=""
              onClick={(event) => {
                onSelect?.();
                handleClick(event);
              }}
              class="block rounded-sm py-1 text-muted transition-colors hover:text-base-content hover:underline"
            >
              {item.text}
            </a>
          </li>
        )}
      </For>
    </ul>
  );

  return (
    <div
      class={`mx-auto w-full space-y-4 ${showToc() && tocOpenDesktop() ? 'max-w-6xl' : 'max-w-4xl'}`}
    >
      <nav
        aria-label="Documentation navigation"
        class="flex flex-wrap items-center justify-between gap-2 border-b border-border pb-3"
      >
        <div class="flex flex-wrap items-center gap-4">
          <a
            href="/"
            onClick={handleAppLinkClick}
            class="inline-flex min-h-10 items-center gap-1.5 text-sm font-medium text-blue-600 hover:underline dark:text-blue-400"
          >
            <ArrowLeft class="h-4 w-4 shrink-0" aria-hidden="true" />
            Back to Pulse
          </a>
          <Show when={!isIndexPage()}>
            <a
              href={docRouteForPath(DOCS_INDEX_PATH)}
              data-doc-link=""
              onClick={handleClick}
              class="inline-flex min-h-10 items-center gap-1 text-sm text-muted hover:text-base-content hover:underline"
            >
              All documentation
            </a>
          </Show>
        </div>
        <Show when={showToc() && !tocOpenDesktop()}>
          <button
            type="button"
            onClick={() => setTocOpenDesktop(true)}
            class="hidden lg:inline-flex items-center gap-1.5 rounded-md border border-border bg-surface px-2.5 py-1.5 text-xs font-medium text-muted hover:bg-surface-hover hover:text-base-content"
          >
            <List class="h-3.5 w-3.5" aria-hidden="true" />
            Show contents
          </button>
        </Show>
      </nav>

      <PageHeader
        title={title()}
        description="Documentation shipped with this Pulse installation."
      />

      <Show when={isIndexPage() && !waiting() && !loadError() && markdown()}>
        <div class="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
          <SearchInput
            value={indexQuery}
            onChange={setIndexQuery}
            placeholder="Filter guides by topic, platform, or keyword..."
            class="sm:max-w-md"
            typeToSearch={false}
            clearOnFocusedEscape
          />
          <div class="flex items-center gap-2 text-xs text-muted">
            <span role="status">
              {hasIndexQuery()
                ? `Showing ${filteredIndex().matchCount} of ${filteredIndex().totalCount} guides`
                : `${filteredIndex().totalCount} guides`}
            </span>
            <Show when={hasIndexQuery()}>
              <button
                type="button"
                onClick={() => setIndexQuery('')}
                class="font-medium text-blue-600 hover:underline dark:text-blue-400"
              >
                Clear filter
              </button>
            </Show>
          </div>
        </div>
      </Show>

      <Show
        when={!waiting() || loadError()}
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
          <Show when={showToc()}>
            <div class="rounded-lg border border-border bg-surface-alt/60 lg:hidden">
              <button
                type="button"
                aria-expanded={tocOpenMobile()}
                onClick={() => setTocOpenMobile((open) => !open)}
                class="flex min-h-11 w-full items-center justify-between gap-2 px-3 py-2 text-left text-xs font-semibold uppercase tracking-wide text-base-content"
              >
                <span class="flex items-center gap-2">
                  <List class="h-4 w-4 text-muted" aria-hidden="true" />
                  On this page ({tocItems().length})
                </span>
                <Show
                  when={tocOpenMobile()}
                  fallback={<ChevronDown class="h-4 w-4 text-muted" aria-hidden="true" />}
                >
                  <ChevronUp class="h-4 w-4 text-muted" aria-hidden="true" />
                </Show>
              </button>
              <Show when={tocOpenMobile()}>
                <nav aria-label="On this page" class="border-t border-border px-3 py-2.5">
                  {renderTocList(() => setTocOpenMobile(false))}
                </nav>
              </Show>
            </div>
          </Show>

          <div
            class={
              showToc() && tocOpenDesktop()
                ? 'lg:grid lg:grid-cols-[minmax(0,1fr)_15rem] lg:items-start lg:gap-8'
                : undefined
            }
          >
            <div class="min-w-0">
              <Show
                when={!showIndexFilterEmpty()}
                fallback={
                  <div class="rounded-lg border border-border bg-surface-alt/60 p-6 text-center space-y-2">
                    <p class="text-sm text-muted">
                      No documentation guides match &ldquo;{indexQuery().trim()}&rdquo;.
                    </p>
                    <button
                      type="button"
                      onClick={() => setIndexQuery('')}
                      class="text-sm font-medium text-blue-600 hover:underline dark:text-blue-400"
                    >
                      Clear filter
                    </button>
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
                  innerHTML={displayedHtml()}
                />
              </Show>
            </div>

            <Show when={showToc() && tocOpenDesktop()}>
              <aside class="hidden lg:block lg:sticky lg:top-6">
                <div class="rounded-lg border border-border bg-surface-alt/60 p-4">
                  <div class="mb-2 flex items-center justify-between gap-2">
                    <span class="text-xs font-semibold uppercase tracking-wide text-muted">
                      On this page
                    </span>
                    <button
                      type="button"
                      onClick={() => setTocOpenDesktop(false)}
                      class="text-xs text-muted hover:text-base-content hover:underline"
                    >
                      Hide
                    </button>
                  </div>
                  <nav
                    aria-label="On this page"
                    class="max-h-[calc(100vh-10rem)] overflow-y-auto pr-1"
                  >
                    {renderTocList()}
                  </nav>
                </div>
              </aside>
            </Show>
          </div>
        </Show>
      </Show>
    </div>
  );
}
