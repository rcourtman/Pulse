import { JSX, Show, createEffect, splitProps } from 'solid-js';
import { useKioskMode } from '@/hooks/useKioskMode';

type PageHeaderProps = {
  id?: string;
  title: JSX.Element;
  description?: JSX.Element;
  descriptionVisibility?: 'desktop' | 'always';
  updateDocumentTitle?: boolean;
  titleMeta?: JSX.Element;
  actions?: JSX.Element;
  class?: string;
  titleClass?: string;
  descriptionClass?: string;
} & Omit<JSX.HTMLAttributes<HTMLDivElement>, 'title'>;

export function PageHeader(props: PageHeaderProps) {
  const [local, rest] = splitProps(props, [
    'id',
    'title',
    'description',
    'descriptionVisibility',
    'updateDocumentTitle',
    'titleMeta',
    'actions',
    'class',
    'titleClass',
    'descriptionClass',
  ]);

  const kioskMode = useKioskMode();

  // When the page header title is a plain string, mirror it into
  // document.title so browser tabs and screen-reader page-title
  // announcements identify the specific surface (e.g. "Alert History"
  // or "General") instead of just the top-level tab name set by
  // AppLayout. Non-string titles (rich JSX) fall back to whatever
  // AppLayout already set.
  createEffect(() => {
    if (
      local.updateDocumentTitle !== false &&
      typeof local.title === 'string' &&
      local.title.trim()
    ) {
      document.title = `${local.title.trim()} · Pulse`;
    }
  });

  const descriptionClass = () =>
    local.descriptionVisibility === 'always'
      ? `mt-1 text-sm font-medium text-muted ${local.descriptionClass ?? ''}`.trim()
      : `mt-1 hidden text-sm font-medium text-muted sm:block ${local.descriptionClass ?? ''}`.trim();

  return (
    <Show when={!kioskMode()}>
      <div
        class={`flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between ${local.class ?? ''}`.trim()}
        {...rest}
      >
        <div class="min-w-0">
          <div class="flex items-center gap-3">
            <h1
              id={local.id}
              class={`text-2xl font-bold tracking-tight text-base-content ${local.titleClass ?? ''}`.trim()}
            >
              {local.title}
            </h1>
            <Show when={local.titleMeta}>{local.titleMeta}</Show>
          </div>
          <Show when={local.description}>
            <p class={descriptionClass()}>{local.description}</p>
          </Show>
        </div>
        <Show when={local.actions}>
          <div class="w-full sm:w-auto">{local.actions}</div>
        </Show>
      </div>
    </Show>
  );
}

export default PageHeader;
