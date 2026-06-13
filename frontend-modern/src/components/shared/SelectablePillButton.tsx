import { JSX, mergeProps, splitProps } from 'solid-js';
import { getSelectablePillButtonClass, type SelectablePillButtonSize } from './selectablePillModel';

export interface SelectablePillButtonProps extends Omit<
  JSX.ButtonHTMLAttributes<HTMLButtonElement>,
  'aria-pressed'
> {
  active: boolean;
  size?: SelectablePillButtonSize;
  class?: string;
  children?: JSX.Element;
}

export function SelectablePillButton(props: SelectablePillButtonProps) {
  const merged = mergeProps({ type: 'button' as const, size: 'md' as const }, props);
  const [local, rest] = splitProps(merged, ['active', 'size', 'class', 'children', 'disabled']);

  return (
    <button
      {...rest}
      class={getSelectablePillButtonClass({
        active: local.active,
        size: local.size,
        class: local.class,
      })}
      aria-pressed={local.active ? 'true' : 'false'}
      disabled={local.disabled}
    >
      {local.children}
    </button>
  );
}

export default SelectablePillButton;
