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
  ]);

  return (
    <Card
      padding="none"
      tone={local.tone ?? 'default'}
      class={`overflow-hidden border border-gray-200 dark:border-gray-700 ${local.class ?? ''}`.trim()}
      border={false}
      {...rest}
    >
      <div class="bg-blue-50 dark:bg-blue-900/20 px-6 py-4 border-b border-gray-200 dark:border-gray-700">
        <div class="flex items-center gap-3">
          <Show when={local.icon}>
            <div class="p-2 bg-blue-100 dark:bg-blue-900/40 rounded-lg text-blue-600 dark:text-blue-300">
              {local.icon}
            </div>
          </Show>
          <SectionHeader
            title={local.title}
            description={local.description}
            size="sm"
            class="flex-1"
          />
          <Show when={local.action}>
            <div>{local.action}</div>
          </Show>
        </div>
      </div>
      <div class={`p-6 ${local.bodyClass ?? 'space-y-6'}`}>{local.children}</div>
    </Card>
  );
}

export default SettingsPanel;
