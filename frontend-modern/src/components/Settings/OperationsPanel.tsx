import { JSX } from 'solid-js';
import SettingsPanel from '@/components/shared/SettingsPanel';

type OperationsPanelProps = {
  title: JSX.Element;
  description?: JSX.Element;
  icon?: JSX.Element;
  action?: JSX.Element;
  class?: string;
  children: JSX.Element;
};

export function OperationsPanel(props: OperationsPanelProps) {
  return (
    <SettingsPanel
      title={props.title}
      description={props.description}
      icon={props.icon}
      action={props.action}
      class={props.class}
      noPadding
      bodyClass="divide-y divide-border"
    >
      {props.children}
    </SettingsPanel>
  );
}

export default OperationsPanel;
