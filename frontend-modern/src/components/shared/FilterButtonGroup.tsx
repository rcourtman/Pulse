import { For } from 'solid-js';

export interface FilterOption<T extends string | number> {
  value: T;
  label: string;
  icon?: (props: { class?: string }) => any;
  disabled?: boolean;
}

type FilterButtonGroupVariant = 'default' | 'settings' | 'prominent';

interface FilterButtonGroupProps<T extends string | number> {
  options: FilterOption<T>[];
  value: T;
  onChange: (value: T) => void;
  class?: string;
  variant?: FilterButtonGroupVariant;
  disabled?: boolean;
}

const groupClassByVariant: Record<FilterButtonGroupVariant, string> = {
  default: 'flex p-1 space-x-1 bg-surface-alt rounded-md overflow-x-auto scrollbar-hide',
  settings: 'flex items-center gap-1 bg-surface-alt rounded-md p-1 overflow-x-auto scrollbar-hide',
  prominent: 'grid grid-cols-1 gap-2',
};

function buttonClass(
  variant: FilterButtonGroupVariant,
  active: boolean,
  disabled: boolean,
): string {
  if (variant === 'settings') {
    return [
      'flex items-center justify-center gap-1.5 min-h-10 sm:min-h-9 px-3 py-2 text-sm rounded-md transition-all whitespace-nowrap outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2',
      active ? 'bg-surface text-base-content shadow-sm' : 'text-muted hover:text-base-content',
      disabled ? 'opacity-60 cursor-not-allowed' : '',
    ].join(' ');
  }

  if (variant === 'prominent') {
    return [
      'w-full flex items-center justify-center gap-2 min-h-10 rounded-md border px-4 py-2.5 text-sm font-medium transition-all whitespace-nowrap outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2',
      active
        ? 'bg-blue-50 border-blue-500 text-blue-700 dark:bg-blue-900 dark:text-blue-300 dark:border-blue-500'
        : 'border-border text-base-content hover:bg-surface-alt',
      disabled ? 'opacity-60 cursor-not-allowed' : '',
    ].join(' ');
  }

  return [
    'flex flex-1 justify-center sm:flex-none sm:justify-start items-center gap-2 px-3 sm:px-4 py-2.5 sm:py-2 text-sm font-medium rounded-md transition-all whitespace-nowrap outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2',
    active
      ? 'bg-surface border border-border text-blue-600 dark:text-blue-400 shadow-sm'
      : 'text-muted border border-transparent hover:text-base-content hover:bg-surface-hover',
    disabled ? 'opacity-60 cursor-not-allowed' : '',
  ].join(' ');
}

export function FilterButtonGroup<T extends string | number>(props: FilterButtonGroupProps<T>) {
  const variant = () => props.variant ?? 'default';

  return (
    <div
      class={`${groupClassByVariant[variant()]} touch-scroll ${props.class ?? ''}`.trim()}
      role="group"
      aria-label="Filter Options"
    >
      <For each={props.options}>
        {(option) => {
          const isActive = () => option.value === props.value;
          const isDisabled = () => props.disabled || option.disabled || false;
          const Icon = option.icon;

          return (
            <button
              type="button"
              onClick={() => props.onChange(option.value)}
              class={buttonClass(variant(), isActive(), isDisabled())}
              aria-pressed={isActive()}
              disabled={isDisabled()}
            >
              {Icon && <Icon class="w-4 h-4 sm:w-[18px] sm:h-[18px]" />}
              {variant() === 'prominent' ? (
                <span>{option.label}</span>
              ) : (
                <>
                  <span class="hidden sm:inline">{option.label}</span>
                  <span class="sm:hidden">{option.label.split(' ').pop()}</span>
                </>
              )}
            </button>
          );
        }}
      </For>
    </div>
  );
}

export default FilterButtonGroup;
