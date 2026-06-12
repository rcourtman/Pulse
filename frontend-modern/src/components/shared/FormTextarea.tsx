import { Show, createEffect, createUniqueId, splitProps } from 'solid-js';
import type { Component, JSX } from 'solid-js';
import { formField, formHelpText, formLabel, formTextarea } from '@/components/shared/Form';

interface FormTextareaProps extends JSX.TextareaHTMLAttributes<HTMLTextAreaElement> {
  label: JSX.Element;
  fieldBaseClass?: string;
  fieldClass?: string;
  labelClass?: string;
  textareaBaseClass?: string;
  textareaClass?: string;
  help?: JSX.Element;
  helpClass?: string;
}

const joinClass = (...parts: Array<string | undefined>) => parts.filter(Boolean).join(' ');

export const FormTextarea: Component<FormTextareaProps> = (props) => {
  const [local, textareaProps] = splitProps(props, [
    'fieldBaseClass',
    'fieldClass',
    'help',
    'helpClass',
    'id',
    'label',
    'labelClass',
    'aria-describedby',
    'class',
    'textareaBaseClass',
    'textareaClass',
    'value',
  ]);
  let textareaElement: HTMLTextAreaElement | undefined;
  const generatedId = `form-textarea-${createUniqueId()}`;
  const textareaId = () => (typeof local.id === 'string' && local.id ? local.id : generatedId);
  const helpId = () => `${textareaId()}-help`;
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

  createEffect(() => {
    const value = local.value;
    if (!textareaElement || value === undefined || value === null) return;

    const nextValue = String(value);
    if (textareaElement.value !== nextValue) {
      textareaElement.value = nextValue;
    }
  });

  return (
    <div class={joinClass(local.fieldBaseClass ?? formField, local.fieldClass)}>
      <label for={textareaId()} class={joinClass(formLabel, local.labelClass)}>
        {local.label}
      </label>
      <textarea
        ref={(element) => {
          textareaElement = element;
        }}
        {...textareaProps}
        id={textareaId()}
        aria-describedby={describedBy()}
        class={joinClass(local.textareaBaseClass ?? formTextarea, local.class, local.textareaClass)}
      />
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
