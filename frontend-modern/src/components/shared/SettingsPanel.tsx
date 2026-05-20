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
      padding="none"
      tone={local.tone ?? 'default'}
      class={`overflow-hidden border border-border ${local.class ?? ''}`.trim()}
      border={false}
      {...rest}
    >
      <div class="px-3 py-3 sm:px-6 sm:py-4 border-b border-border bg-surface-alt">
        <div class="flex flex-col gap-3 sm:flex-row sm:items-center">
          <div class="flex min-w-0 flex-col gap-1 flex-1">
            <h2
              id={local.titleId}
              class="text-sm sm:text-base tracking-tight font-semibold text-base-content dark:text-slate-100"
            >
              {local.title}
            </h2>
            <Show when={local.description}>
              <p class="text-xs sm:text-sm text-muted dark:text-slate-200">{local.description}</p>
            </Show>
          </div>
          <Show when={local.action}>
            <div class="w-full sm:w-auto">{local.action}</div>
          </Show>
        </div>
      </div>
      <div
        class={`${local.noPadding ? '' : 'p-4 sm:p-6'} ${local.bodyClass ?? (local.noPadding ? '' : 'space-y-6')}`}
      >
        {local.children}
      </div>
    </Card>
  );
}

export default SettingsPanel;
