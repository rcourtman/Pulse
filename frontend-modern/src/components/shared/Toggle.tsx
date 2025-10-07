import { JSX } from 'solid-js';

type ToggleSize = 'xs' | 'sm' | 'md';

interface ToggleChangeEvent {
  currentTarget: {
    checked: boolean;
  };
}

interface BaseToggleProps {
  checked: boolean;
  disabled?: boolean;
  onToggle?: () => void;
  onChange?: (event: ToggleChangeEvent) => void;
  size?: ToggleSize;
  class?: string;
  title?: string;
  ariaLabel?: string;
  checkedClass?: string;
  uncheckedClass?: string;
  disabledClass?: string;
  knobClass?: string;
}

const sizeConfig: Record<ToggleSize, { track: string; knob: string; translate: string }> = {
  xs: { track: 'h-4 w-8', knob: 'h-3 w-3', translate: '14px' },
  sm: { track: 'h-5 w-10', knob: 'h-4 w-4', translate: '18px' },
  md: { track: 'h-6 w-11', knob: 'h-5 w-5', translate: '20px' },
};

export function TogglePrimitive(props: BaseToggleProps): JSX.Element {
  const size = props.size ?? 'sm';
  const config = sizeConfig[size];
  const isDisabled = () => Boolean(props.disabled);
  const checkedClass = props.checkedClass ?? 'bg-emerald-500/80 border-emerald-600/70 dark:bg-emerald-500/60 dark:border-emerald-500/70';
  const uncheckedClass = props.uncheckedClass ?? 'bg-rose-500/80 border-rose-600/70 dark:bg-rose-500/60 dark:border-rose-500/70';
  const disabledClass = props.disabledClass ?? 'bg-slate-400/60 border-slate-500/70 dark:bg-slate-600/60 dark:border-slate-600/70 cursor-not-allowed opacity-60';
  const knobBase = props.knobClass ?? 'bg-white shadow';

  const handleClick = () => {
    if (isDisabled()) return;
    const next = !props.checked;
    props.onToggle?.();
    props.onChange?.({ currentTarget: { checked: next } });
  };

  return (
    <button
      type="button"
      class={`relative inline-flex ${config.track} items-center justify-start rounded-full border transition-colors duration-200 focus:outline-none focus-visible:ring-2 focus-visible:ring-sky-300/70 ${
        isDisabled() ? disabledClass : props.checked ? checkedClass : uncheckedClass
      } ${props.class ?? ''}`.trim()}
      onClick={handleClick}
      disabled={props.disabled}
      title={props.title}
      aria-pressed={props.checked ? 'true' : 'false'}
      aria-label={props.ariaLabel}
    >
      <span
        class={`absolute left-[3px] inline-block ${config.knob} rounded-full transition-transform duration-200 ${knobBase} ${
          isDisabled() ? 'opacity-85' : ''
        } ${props.checked ? '' : ''}`}
        style={{ transform: props.checked ? `translateX(${config.translate})` : 'translateX(0)' }}
      />
    </button>
  );
}

interface LabeledToggleProps extends BaseToggleProps {
  label?: JSX.Element;
  description?: JSX.Element;
  containerClass?: string;
}

export function Toggle(props: LabeledToggleProps) {
  const size = props.size ?? 'md';
  return (
    <label class={`flex items-center gap-3 ${props.containerClass ?? ''}`.trim()}>
      <TogglePrimitive {...props} size={size} />
      {(props.label || props.description) && (
        <span class="flex flex-col text-sm text-gray-700 dark:text-gray-300">
          {props.label}
          {props.description && (
            <span class="text-xs text-gray-500 dark:text-gray-400">{props.description}</span>
          )}
        </span>
      )}
    </label>
  );
}

export default Toggle;
