import { JSX, Show, createSignal, onCleanup, onMount, splitProps } from 'solid-js';
import { isKioskMode, subscribeToKioskMode } from '@/utils/url';

type PageHeaderProps = {
  id?: string;
  title: JSX.Element;
  description?: JSX.Element;
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
    'titleMeta',
    'actions',
    'class',
    'titleClass',
    'descriptionClass',
  ]);

  const [kiosk, setKiosk] = createSignal(isKioskMode());
  onMount(() => {
    const unsubscribe = subscribeToKioskMode(setKiosk);
    onCleanup(unsubscribe);
  });

  return (
    <Show when={!kiosk()}>
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
            <p class={`mt-1 text-sm font-medium text-muted ${local.descriptionClass ?? ''}`.trim()}>
              {local.description}
            </p>
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
