import { For, type JSX } from 'solid-js';

export type SelectionCardTone = 'accent' | 'success';
type SelectionCardGroupVariant = 'compact' | 'detail';

export interface SelectionCardOption<T extends string | number> {
  value: T;
  title: string;
  description?: string;
  icon?: (props: { active: boolean }) => JSX.Element;
  tone?: SelectionCardTone;
  disabled?: boolean;
}

interface SelectionCardGroupProps<T extends string | number> {
  options: SelectionCardOption<T>[];
  value: T;
  onChange: (value: T) => void;
  class?: string;
  variant?: SelectionCardGroupVariant;
  disabled?: boolean;
}

const groupClassByVariant: Record<SelectionCardGroupVariant, string> = {
  compact: 'grid grid-cols-2 gap-2',
  detail: 'grid grid-cols-1 gap-3',
};

function activeCardClass(tone: SelectionCardTone): string {
  if (tone === 'success') {
    return 'border-green-500 bg-green-50 dark:bg-green-900';
  }
  return 'border-blue-500 bg-blue-50 dark:bg-blue-900';
}

function inactiveCardClass(variant: SelectionCardGroupVariant): string {
  if (variant === 'compact') {
    return 'border-border hover:border-blue-300';
  }
  return 'border-border hover:border-border';
}

function buttonClass(
  variant: SelectionCardGroupVariant,
  tone: SelectionCardTone,
  active: boolean,
  disabled: boolean,
): string {
  const base =
    variant === 'detail'
      ? 'p-4 rounded-md border-2 transition-all text-left'
      : 'p-3 rounded-md border-2 transition-all text-center';

  return [
    base,
    active ? activeCardClass(tone) : inactiveCardClass(variant),
    disabled ? 'disabled:opacity-50 disabled:cursor-not-allowed' : '',
  ].join(' ');
}

function iconContainerClass(tone: SelectionCardTone, active: boolean): string {
  const activeClass =
    tone === 'success' ? 'bg-green-100 dark:bg-green-800' : 'bg-blue-100 dark:bg-blue-800';
  return ['p-2 rounded-md', active ? activeClass : 'bg-surface-alt'].join(' ');
}

function titleClass(
  variant: SelectionCardGroupVariant,
  tone: SelectionCardTone,
  active: boolean,
): string {
  if (variant === 'compact') {
    return 'text-sm font-medium text-base-content';
  }
  if (!active) {
    return 'text-sm font-semibold text-base-content';
  }
  return tone === 'success'
    ? 'text-sm font-semibold text-green-900 dark:text-green-100'
    : 'text-sm font-semibold text-blue-900 dark:text-blue-100';
}

function descriptionClass(variant: SelectionCardGroupVariant): string {
  return variant === 'compact' ? 'text-xs text-slate-500 mt-0.5' : 'text-xs text-muted';
}

export function SelectionCardGroup<T extends string | number>(props: SelectionCardGroupProps<T>) {
  const variant = () => props.variant ?? 'detail';

  return (
    <div
      class={`${groupClassByVariant[variant()]} ${props.class ?? ''}`.trim()}
      role="group"
      aria-label="Selection Cards"
    >
      <For each={props.options}>
        {(option) => {
          const isActive = () => option.value === props.value;
          const isDisabled = () => props.disabled || option.disabled || false;
          const tone = () => option.tone ?? 'accent';

          return (
            <button
              type="button"
              onClick={() => props.onChange(option.value)}
              class={buttonClass(variant(), tone(), isActive(), isDisabled())}
              aria-pressed={isActive()}
              disabled={isDisabled()}
            >
              {variant() === 'detail' ? (
                <div class="flex items-center gap-3">
                  {option.icon && (
                    <div class={iconContainerClass(tone(), isActive())}>
                      {option.icon({ active: isActive() })}
                    </div>
                  )}
                  <div>
                    <p class={titleClass(variant(), tone(), isActive())}>{option.title}</p>
                    {option.description && (
                      <p class={descriptionClass(variant())}>{option.description}</p>
                    )}
                  </div>
                </div>
              ) : (
                <div>
                  <div class={titleClass(variant(), tone(), isActive())}>{option.title}</div>
                  {option.description && (
                    <div class={descriptionClass(variant())}>{option.description}</div>
                  )}
                </div>
              )}
            </button>
          );
        }}
      </For>
    </div>
  );
}

export default SelectionCardGroup;
