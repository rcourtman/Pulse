import { For, Show, createSignal, createUniqueId, type Component } from 'solid-js';

import {
  formatAlertSeverityLabel,
  getAlertSeverityBadgeClass,
} from '@/utils/alertSeverityPresentation';

import { Button } from './Button';

export interface DrawerAttentionItem {
  id: string;
  message: string;
  subject?: string;
  metric?: string;
  severity?: string;
  acknowledged?: boolean;
}

interface DrawerAttentionSectionProps {
  items: DrawerAttentionItem[];
}

const COLLAPSED_ITEM_LIMIT = 3;

export const DrawerAttentionSection: Component<DrawerAttentionSectionProps> = (props) => {
  const headingId = createUniqueId();
  const [expanded, setExpanded] = createSignal(false);
  const hasOverflow = () => props.items.length > COLLAPSED_ITEM_LIMIT;
  const hiddenCount = () => Math.max(0, props.items.length - COLLAPSED_ITEM_LIMIT);
  const visibleItems = () =>
    expanded() ? props.items : props.items.slice(0, COLLAPSED_ITEM_LIMIT);

  return (
    <Show when={props.items.length > 0}>
      <section
        data-testid="drawer-attention-section"
        aria-labelledby={headingId}
        class="w-full overflow-hidden rounded-md border border-border bg-surface lg:max-w-3xl"
      >
        <header class="flex items-center justify-between gap-3 border-b border-border bg-surface-alt px-3 py-2">
          <h3
            id={headingId}
            class="text-[11px] font-semibold uppercase tracking-wide text-base-content"
          >
            Needs attention
          </h3>
          <span class="shrink-0 text-[11px] tabular-nums text-muted">
            {props.items.length} active
          </span>
        </header>

        <ul class="divide-y divide-border" aria-label="Active alerts">
          <For each={visibleItems()}>
            {(item) => (
              <li class="grid grid-cols-[minmax(0,1fr)_auto] gap-x-3 gap-y-1 px-3 py-2.5">
                <div class="min-w-0">
                  <Show when={item.subject || item.metric}>
                    <p class="flex min-w-0 flex-wrap items-baseline gap-x-1.5 text-xs">
                      <Show when={item.subject}>
                        <span class="truncate font-semibold text-base-content" title={item.subject}>
                          {item.subject}
                        </span>
                      </Show>
                      <Show when={item.subject && item.metric}>
                        <span class="text-muted" aria-hidden="true">
                          ·
                        </span>
                      </Show>
                      <Show when={item.metric}>
                        <span class="text-muted">{item.metric}</span>
                      </Show>
                    </p>
                  </Show>
                  <p class="break-words text-xs leading-5 text-base-content" title={item.message}>
                    {item.message}
                  </p>
                </div>
                <span
                  class={
                    item.acknowledged
                      ? 'inline-flex shrink-0 self-start items-center rounded bg-surface-alt px-1.5 py-0.5 text-[10px] font-semibold uppercase text-muted'
                      : `${getAlertSeverityBadgeClass(item.severity || 'warning')} self-start`
                  }
                >
                  {item.acknowledged
                    ? 'Acknowledged'
                    : formatAlertSeverityLabel(item.severity, 'Warning')}
                </span>
              </li>
            )}
          </For>
        </ul>

        <Show when={hasOverflow()}>
          <div class="border-t border-border px-2 py-1">
            <Button
              variant="ghost"
              size="xs"
              class="w-full justify-start text-muted hover:text-base-content"
              aria-expanded={expanded()}
              onClick={() => setExpanded((current) => !current)}
            >
              {expanded()
                ? 'Show fewer alerts'
                : `Show ${hiddenCount()} more ${hiddenCount() === 1 ? 'alert' : 'alerts'}`}
            </Button>
          </div>
        </Show>
      </section>
    </Show>
  );
};
