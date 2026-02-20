import { JSX, Show, splitProps } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { SectionHeader } from '@/components/shared/SectionHeader';

type SettingsPanelProps = {
  title: JSX.Element;
  description?: JSX.Element;
  action?: JSX.Element;
  icon?: JSX.Element;
  bodyClass?: string;
  tone?: 'default' | 'muted' | 'info' | 'success' | 'warning' | 'danger';
  padding?: 'none' | 'sm' | 'md' | 'lg';
  noPadding?: boolean;
} & Omit<JSX.HTMLAttributes<HTMLDivElement>, 'title'>;

export function SettingsPanel(props: SettingsPanelProps) {
  const [local, rest] = splitProps(props, [
    'title',
    'description',
    'action',
    'icon',
    'bodyClass',
    'children',
    'class',
    'tone',
    'padding',
    'noPadding'
  ]);

  return (
    <Card
      padding="none"
      tone={local.tone ?? 'default'}
      class={`overflow-hidden border border-slate-200 dark:border-slate-800 ${local.class ?? ''}`.trim()}
      border={false}
      {...rest}
    >
      <div class="px-3 py-3 sm:px-6 sm:py-4 border-b border-slate-200 dark:border-slate-800 bg-slate-50/50 dark:bg-slate-800/50">
        <div class="flex flex-col gap-3 sm:flex-row sm:items-center">
          <div class="flex min-w-0 items-center gap-3 flex-1">
            <Show when={local.icon}>
              <div class="text-slate-400 dark:text-slate-500">
                {local.icon}
              </div>
            </Show>
            <SectionHeader
              title={local.title}
              description={local.description}
              size="sm"
              class="min-w-0 flex-1"
            />
          </div>
          <Show when={local.action}>
            <div class="w-full sm:w-auto">{local.action}</div>
          </Show>
        </div>
      </div>
      <div class={`${local.noPadding ? '' : 'p-4 sm:p-6'} ${local.bodyClass ?? (local.noPadding ? '' : 'space-y-6')}`}>
        {local.children}
      </div>
    </Card>
  );
}

export default SettingsPanel;
