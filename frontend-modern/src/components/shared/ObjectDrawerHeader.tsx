import ChevronUpIcon from 'lucide-solid/icons/chevron-up';
import { Show, splitProps, type JSX } from 'solid-js';

export interface ObjectDrawerHeaderProps extends JSX.HTMLAttributes<HTMLDivElement> {
  actions?: JSX.Element;
  children: JSX.Element;
  collapseLabel: string;
  onCollapse: () => void;
}

export function ObjectDrawerHeader(props: ObjectDrawerHeaderProps) {
  const [local, rest] = splitProps(props, [
    'actions',
    'children',
    'class',
    'collapseLabel',
    'onCollapse',
  ]);

  return (
    <div
      {...rest}
      class={`relative -m-1 flex min-h-11 min-w-0 items-start justify-between gap-3 rounded-md p-1 sm:min-h-8 ${local.class ?? ''}`.trim()}
    >
      <button
        type="button"
        class="absolute inset-0 z-0 cursor-pointer rounded-md transition-colors hover:bg-surface-hover focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
        onClick={local.onCollapse}
        aria-label={local.collapseLabel}
        title={local.collapseLabel}
      />
      <div class="pointer-events-none relative z-10 min-w-0 flex-1">{local.children}</div>
      <Show when={local.actions}>
        <div class="pointer-events-none relative z-10 flex shrink-0 items-center gap-1.5 [&>*]:pointer-events-auto">
          {local.actions}
        </div>
      </Show>
      <ChevronUpIcon
        class="pointer-events-none relative z-10 mt-1 h-4 w-4 shrink-0 text-muted sm:mt-0.5"
        aria-hidden="true"
      />
    </div>
  );
}
