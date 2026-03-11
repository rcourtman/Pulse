import { For } from 'solid-js';

export interface FilterOption<T extends string> {
  value: T;
  label: string;
  icon?: (props: { class?: string }) => any;
}

interface FilterButtonGroupProps<T extends string> {
  options: FilterOption<T>[];
  value: T;
  onChange: (value: T) => void;
  class?: string;
  variant?: 'default' | 'settings';
}

const groupClassByVariant: Record<NonNullable<FilterButtonGroupProps<string>['variant']>, string> = {
  default: 'flex p-1 space-x-1 bg-surface-alt rounded-md overflow-x-auto scrollbar-hide',
  settings: 'flex items-center gap-1 bg-surface-alt rounded-md p-1 overflow-x-auto scrollbar-hide',
};

function buttonClass(
  variant: NonNullable<FilterButtonGroupProps<string>['variant']>,
  active: boolean,
): string {
  if (variant === 'settings') {
    return [
      'flex items-center justify-center gap-1.5 min-h-10 sm:min-h-9 px-3 py-2 text-sm rounded-md transition-all whitespace-nowrap outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2',
      active ? 'bg-surface text-base-content shadow-sm' : 'text-muted hover:text-base-content',
    ].join(' ');
  }

  return [
    'flex flex-1 justify-center sm:flex-none sm:justify-start items-center gap-2 px-3 sm:px-4 py-2.5 sm:py-2 text-sm font-medium rounded-md transition-all whitespace-nowrap outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2',
    active
      ? 'bg-surface border border-border text-blue-600 dark:text-blue-400 shadow-sm'
      : 'text-muted border border-transparent hover:text-base-content hover:bg-surface-hover',
  ].join(' ');
}

export function FilterButtonGroup<T extends string>(props: FilterButtonGroupProps<T>) {
  const variant = () => props.variant ?? 'default';

  return (
    <div
      class={`${groupClassByVariant[variant()]} ${props.class ?? ''}`.trim()}
      style="-webkit-overflow-scrolling: touch;"
      role="group"
      aria-label="Filter Options"
    >
      <For each={props.options}>
        {(option) => {
          const isActive = () => option.value === props.value;
          const Icon = option.icon;

          return (
            <button
              type="button"
              onClick={() => props.onChange(option.value)}
              class={buttonClass(variant(), isActive())}
              aria-pressed={isActive()}
            >
              {Icon && <Icon class="w-4 h-4 sm:w-[18px] sm:h-[18px]" />}
              <span class="hidden sm:inline">{option.label}</span>
              <span class="sm:hidden">{option.label.split(' ').pop()}</span>
            </button>
          );
        }}
      </For>
    </div>
  );
}

export default FilterButtonGroup;
