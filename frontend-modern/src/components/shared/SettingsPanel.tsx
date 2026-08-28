import { JSX, Show, splitProps } from 'solid-js';
import { Card } from '@/components/shared/Card';

type SettingsPanelProps = {
  title: JSX.Element;
  description?: JSX.Element;
  titleId?: string;
  action?: JSX.Element;
  bodyClass?: string;
  tone?: 'default' | 'muted' | 'info' | 'success' | 'warning' | 'danger';
  padding?: 'none' | 'sm' | 'md' | 'lg';
  noPadding?: boolean;
} & Omit<JSX.HTMLAttributes<HTMLDivElement>, 'title'>;

export function SettingsPanel(props: SettingsPanelProps) {
  const [local, rest] = splitProps(props, [
    'title',
    'description',
    'titleId',
    'action',
    'bodyClass',
    'children',
    'class',
    'tone',
    'padding',
    'noPadding',
  ]);

  return (
    <Card
      data-settings-panel
      padding="none"
      tone={local.tone ?? 'default'}
      class={`overflow-hidden border border-border ${local.class ?? ''}`.trim()}
      border={false}
      {...rest}
    >
      <div class="border-b border-border bg-surface-alt px-2.5 py-2 sm:px-6 sm:py-4">
        <div class="flex items-start gap-2 sm:items-center sm:gap-3">
          <div class="flex min-w-0 flex-1 flex-col gap-0.5 sm:gap-1">
            <h2
              id={local.titleId}
              class="text-[11px] font-semibold uppercase tracking-wide text-base-content dark:text-slate-100 sm:text-base sm:normal-case sm:tracking-tight"
            >
              {local.title}
            </h2>
            <Show when={local.description}>
              <p class="line-clamp-1 text-[11px] text-muted dark:text-slate-200 sm:text-sm">
                {local.description}
              </p>
            </Show>
          </div>
          <Show when={local.action}>
            <div class="w-auto shrink-0">{local.action}</div>
          </Show>
        </div>
      </div>
      <div
        class={`${local.noPadding ? '' : 'p-2.5 sm:p-6'} ${local.bodyClass ?? (local.noPadding ? '' : 'space-y-3 sm:space-y-6')}`}
      >
        {local.children}
      </div>
    </Card>
  );
}

export default SettingsPanel;
