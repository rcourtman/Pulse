import { Component } from 'solid-js';
import { CopyValueButton, type CopyValueButtonProps } from './Button';

export interface CopyableCodeRowProps extends Pick<
  CopyValueButtonProps,
  'copied' | 'label' | 'onCopyValue'
> {
  value: string;
  class?: string;
  codeClass?: string;
}

export const CopyableCodeRow: Component<CopyableCodeRowProps> = (props) => (
  <div
    class={['flex items-start gap-2 rounded bg-surface-alt px-2 py-1.5', props.class]
      .filter(Boolean)
      .join(' ')}
  >
    <code
      class={['min-w-0 flex-1 break-all font-mono text-xs text-base-content', props.codeClass]
        .filter(Boolean)
        .join(' ')}
    >
      {props.value}
    </code>
    <CopyValueButton
      value={props.value}
      copied={props.copied}
      onCopyValue={props.onCopyValue}
      label={props.label}
      variant="ghost"
      size="sm"
    />
  </div>
);

export default CopyableCodeRow;
