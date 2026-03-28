import { Component, For, JSX, splitProps } from 'solid-js';

export interface SubtabOption {
  value: string;
  label: JSX.Element;
  disabled?: boolean;
}

interface SubtabsProps extends Omit<JSX.HTMLAttributes<HTMLDivElement>, 'onChange'> {
  value: string;
  onChange: (value: string) => void;
  tabs: SubtabOption[];
  ariaLabel: string;
  variant?: 'default' | 'control';
  listClass?: string;
  tabClass?: string;
}

export const subtabsShellClass = 'border-b border-border';
export const subtabsListClass = 'flex flex-wrap items-center gap-6';
export const subtabButtonClass =
  'inline-flex min-h-10 items-center border-b-2 px-1 py-2 text-sm font-medium transition-colors';
export const subtabButtonActiveClass = 'border-blue-600 text-base-content';
export const subtabButtonInactiveClass = 'border-transparent text-muted hover:text-base-content';
export const subtabsControlShellClass = 'rounded-md border border-border bg-surface-alt p-0.5';
export const subtabsControlListClass =
  'flex flex-wrap items-center gap-1 overflow-x-auto scrollbar-hide';
export const subtabControlButtonClass =
  'inline-flex min-h-8 whitespace-nowrap rounded-md border border-transparent px-3 py-1.5 text-xs font-medium transition-colors';
export const subtabControlButtonActiveClass =
  'border-border-subtle bg-surface text-base-content shadow-sm';
export const subtabControlButtonInactiveClass =
  'text-muted hover:bg-surface-hover hover:text-base-content';

export const Subtabs: Component<SubtabsProps> = (props) => {
  const [local, divProps] = splitProps(props, [
    'value',
    'onChange',
    'tabs',
    'ariaLabel',
    'variant',
    'class',
    'listClass',
    'tabClass',
  ]);
  const isControlVariant = () => local.variant === 'control';

  return (
    <div
      {...divProps}
      class={`${
        isControlVariant() ? subtabsControlShellClass : subtabsShellClass
      } ${local.class ?? ''}`.trim()}
    >
      <div
        role="tablist"
        aria-label={local.ariaLabel}
        class={`${
          isControlVariant() ? subtabsControlListClass : subtabsListClass
        } ${local.listClass ?? ''}`.trim()}
      >
        <For each={local.tabs}>
          {(tab) => {
            const selected = () => local.value === tab.value;
            return (
              <button
                type="button"
                role="tab"
                aria-selected={selected()}
                tabIndex={selected() ? 0 : -1}
                disabled={tab.disabled}
                onClick={() => local.onChange(tab.value)}
                class={`${
                  isControlVariant() ? subtabControlButtonClass : subtabButtonClass
                } ${
                  selected()
                    ? isControlVariant()
                      ? subtabControlButtonActiveClass
                      : subtabButtonActiveClass
                    : isControlVariant()
                      ? subtabControlButtonInactiveClass
                      : subtabButtonInactiveClass
                } ${local.tabClass ?? ''}`.trim()}
              >
                {tab.label}
              </button>
            );
          }}
        </For>
      </div>
    </div>
  );
};
