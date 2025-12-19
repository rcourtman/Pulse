import { JSX, Show, splitProps } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { SectionHeader } from '@/components/shared/SectionHeader';

type SettingsPanelProps = {
  title: JSX.Element;
  description?: JSX.Element;
  action?: JSX.Element;
  bodyClass?: string;
  tone?: 'default' | 'muted' | 'info' | 'success' | 'warning' | 'danger';
  padding?: 'none' | 'sm' | 'md' | 'lg';
} & Omit<JSX.HTMLAttributes<HTMLDivElement>, 'title'>;

export function SettingsPanel(props: SettingsPanelProps) {
  const [local, rest] = splitProps(props, [
    'title',
    'description',
    'action',
    'bodyClass',
    'children',
    'class',
    'tone',
    'padding',
  ]);

  return (
    <Card
      padding={local.padding ?? 'lg'}
      tone={local.tone ?? 'default'}
      class={`space-y-6 ${local.class ?? ''}`.trim()}
      {...rest}
    >
      <div class="flex flex-col gap-4 md:flex-row md:items-start md:justify-between">
        <SectionHeader
          title={local.title}
          description={local.description}
          size="sm"
          class="flex-1"
        />
        <Show when={local.action}>
          <div class="md:ml-6">{local.action}</div>
        </Show>
      </div>
      <div class={local.bodyClass ?? 'space-y-4'}>{local.children}</div>
    </Card>
  );
}

export default SettingsPanel;
