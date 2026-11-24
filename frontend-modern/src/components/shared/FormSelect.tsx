import { Show, createEffect, createUniqueId, onCleanup, splitProps } from 'solid-js';
import type { Component, JSX } from 'solid-js';
import ChevronDownIcon from 'lucide-solid/icons/chevron-down';
import {
  formField,
  formHelpText,
  formLabel,
  formSelect,
  formSelectCompact,
} from '@/components/shared/Form';

interface FormSelectProps extends JSX.SelectHTMLAttributes<HTMLSelectElement> {
  label: JSX.Element;
  children: JSX.Element;
  fieldBaseClass?: string;
  fieldClass?: string;
  labelClass?: string;
  selectBaseClass?: string;
  selectClass?: string;
  density?: 'default' | 'compact';
  help?: JSX.Element;
  helpClass?: string;
}

const joinClass = (...parts: Array<string | undefined>) => parts.filter(Boolean).join(' ');

const tokenizeClasses = (classList: string) => classList.split(/\s+/).filter(Boolean);

const isCompactSelect = (density: FormSelectProps['density'], tokens: string[]) =>
  density === 'compact' ||
  tokens.includes('text-xs') ||
  tokens.includes('text-[11px]') ||
  tokens.includes('px-2') ||
  tokens.includes('px-2.5');

const resolveSelectWrapperClass = (tokens: string[]) => {
  const hasAutoWidth = tokens.includes('w-auto') || tokens.includes('w-fit');
  const hasFixedWidth = tokens.some((token) => /^w-(?!full$|auto$|fit$)/.test(token));
  const hasFullWidth =
    !hasAutoWidth &&
    !hasFixedWidth &&
    (tokens.includes('w-full') || tokens.includes('form-select'));
  const maxWidthTokens = tokens.filter((token) => /^(?:(?:sm|md|lg|xl|2xl):)?max-w-/.test(token));

  if (hasFullWidth) {
    if (tokens.includes('sm:w-auto')) {
      return joinClass(
        'relative inline-grid w-full max-w-full min-w-0 sm:w-fit',
        ...maxWidthTokens,
      );
    }
    return joinClass('relative grid w-full min-w-0', ...maxWidthTokens);
  }

  return joinClass('relative inline-grid max-w-full min-w-0', ...maxWidthTokens);
};

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
    'ref',
    'aria-describedby',
    'class',
    'selectBaseClass',
    'selectClass',
    'density',
    'value',
  ]);
  let selectElement: HTMLSelectElement | undefined;
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
  const rawSelectClass = () =>
    joinClass(
      local.selectBaseClass ?? (local.density === 'compact' ? formSelectCompact : formSelect),
      local.class,
      local.selectClass,
    );
  const selectTokens = () => tokenizeClasses(rawSelectClass());
  const showChevron = () =>
    !selectProps.multiple &&
    !(Number(selectProps.size) > 1) &&
    !selectTokens().includes('opacity-0');
  const compactSelect = () => isCompactSelect(local.density, selectTokens());
  const wrapperClass = () => resolveSelectWrapperClass(selectTokens());
  const styledSelectClass = () =>
    joinClass(
      rawSelectClass(),
      'col-start-1 row-start-1 appearance-none bg-none!',
      compactSelect() ? 'pr-8!' : 'pr-9!',
    );
  const chevronClass = () => {
    const tokens = selectTokens();
    const topMarginToken = tokens.find((token) => /^mt-/.test(token));
    return joinClass(
      'pointer-events-none col-start-1 row-start-1 self-center justify-self-end text-muted transition-opacity',
      compactSelect() ? 'mr-2 h-3.5 w-3.5' : 'mr-2.5 h-4 w-4',
      topMarginToken,
      selectProps.disabled ? 'opacity-50' : undefined,
      tokens.includes('form-select-arrow') ? 'forced-colors:hidden' : undefined,
    );
  };
  const attachSelectRef = (element: HTMLSelectElement) => {
    selectElement = element;
    if (typeof local.ref === 'function') {
      local.ref(element);
    }
  };

  createEffect(() => {
    const value = local.value;
    if (!selectElement || value === undefined || value === null) return;

    const applyValue = () => {
      if (!selectElement) return;

      if (Array.isArray(value)) {
        const selectedValues = new Set(value.map(String));
        for (const option of Array.from(selectElement.options)) {
          option.selected = selectedValues.has(option.value);
        }
        return;
      }

      const nextValue = String(value);
      const optionValues = Array.from(selectElement.options, (option) => option.value);
      const resolvedValue = optionValues.includes(nextValue) ? nextValue : '';
      if (selectElement.value !== resolvedValue) {
        selectElement.value = resolvedValue;
      }
    };

    applyValue();

    if (typeof MutationObserver !== 'function') return;

    const observer = new MutationObserver(() => applyValue());
    observer.observe(selectElement, { childList: true, subtree: true });
    onCleanup(() => observer.disconnect());
  });

  return (
    <div class={joinClass(local.fieldBaseClass ?? formField, local.fieldClass)}>
      <label for={selectId()} class={joinClass(formLabel, local.labelClass)}>
        {local.label}
      </label>
      <Show
        when={showChevron()}
        fallback={
          <select
            ref={attachSelectRef}
            {...selectProps}
            id={selectId()}
            aria-describedby={describedBy()}
            class={rawSelectClass()}
          >
            {local.children}
          </select>
        }
      >
        <div class={wrapperClass()}>
          <select
            ref={attachSelectRef}
            {...selectProps}
            id={selectId()}
            aria-describedby={describedBy()}
            class={styledSelectClass()}
          >
            {local.children}
          </select>
          <ChevronDownIcon aria-hidden="true" class={chevronClass()} />
        </div>
      </Show>
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
