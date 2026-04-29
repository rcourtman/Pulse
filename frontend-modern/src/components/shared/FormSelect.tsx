import { Show, createUniqueId, splitProps } from 'solid-js';
import type { Component, JSX } from 'solid-js';
import { formField, formHelpText, formLabel, formSelect } from '@/components/shared/Form';

interface FormSelectProps extends JSX.SelectHTMLAttributes<HTMLSelectElement> {
  label: JSX.Element;
  children: JSX.Element;
  fieldBaseClass?: string;
  fieldClass?: string;
  labelClass?: string;
  selectBaseClass?: string;
  selectClass?: string;
  help?: JSX.Element;
  helpClass?: string;
}

const joinClass = (...parts: Array<string | undefined>) => parts.filter(Boolean).join(' ');

export const FormSelect: Component<FormSelectProps> = (props) => {
  const [local, selectProps] = splitProps(props, [
    'children',
    'fieldBaseClass',
    'fieldClass',
    'help',
    'helpClass',
    'id',
    'label',
    'labelClass',
    'aria-describedby',
    'class',
    'selectBaseClass',
    'selectClass',
  ]);
  const generatedId = `form-select-${createUniqueId()}`;
  const selectId = () => (typeof local.id === 'string' && local.id ? local.id : generatedId);
  const helpId = () => `${selectId()}-help`;
  const describedBy = () => {
    const existing = local['aria-describedby'];
    const ids = [
      typeof existing === 'string' ? existing : undefined,
      local.help ? helpId() : undefined,
    ]
      .filter(Boolean)
      .join(' ');

    return ids || undefined;
  };

  return (
    <div class={joinClass(local.fieldBaseClass ?? formField, local.fieldClass)}>
      <label for={selectId()} class={joinClass(formLabel, local.labelClass)}>
        {local.label}
      </label>
      <select
        {...selectProps}
        id={selectId()}
        aria-describedby={describedBy()}
        class={joinClass(local.selectBaseClass ?? formSelect, local.class, local.selectClass)}
      >
        {local.children}
      </select>
      <Show when={local.help}>
        {(help) => (
          <p id={helpId()} class={joinClass(formHelpText, local.helpClass)}>
            {help()}
          </p>
        )}
      </Show>
    </div>
  );
};
