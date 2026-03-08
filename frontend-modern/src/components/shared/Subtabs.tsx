import { Component, For, JSX, splitProps } from 'solid-js';

export interface SubtabOption {
  value: string;
  label: JSX.Element;
  disabled?: boolean;
}

interface SubtabsProps extends JSX.HTMLAttributes<HTMLDivElement> {
  value: string;
  onChange: (value: string) => void;
  tabs: SubtabOption[];
  ariaLabel: string;
  listClass?: string;
  tabClass?: string;
}

export const subtabsShellClass = 'border-b border-border';
export const subtabsListClass = 'flex flex-wrap items-center gap-6';
export const subtabButtonClass =
  'inline-flex min-h-10 items-center border-b-2 px-1 py-2 text-sm font-medium transition-colors';
export const subtabButtonActiveClass = 'border-blue-600 text-base-content';
export const subtabButtonInactiveClass = 'border-transparent text-muted hover:text-base-content';

export const Subtabs: Component<SubtabsProps> = (props) => {
  const [local, divProps] = splitProps(props, [
    'value',
    'onChange',
    'tabs',
    'ariaLabel',
    'class',
    'listClass',
    'tabClass',
  ]);

  return (
    <div {...divProps} class={`${subtabsShellClass} ${local.class ?? ''}`.trim()}>
      <div
        role="tablist"
        aria-label={local.ariaLabel}
        class={`${subtabsListClass} ${local.listClass ?? ''}`.trim()}
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
                class={`${subtabButtonClass} ${
                  selected() ? subtabButtonActiveClass : subtabButtonInactiveClass
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
