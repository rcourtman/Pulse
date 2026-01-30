import { JSX } from 'solid-js';

type ToggleSize = 'xs' | 'sm' | 'md';

export interface ToggleChangeEvent {
  currentTarget: {
    checked: boolean;
  };
  preventDefault: () => void;
  stopPropagation: () => void;
  readonly defaultPrevented: boolean;
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
}

const sizeConfig: Record<ToggleSize, { track: string; knob: string; translate: string }> = {
  xs: { track: 'h-4 w-7', knob: 'h-3 w-3', translate: '12px' },
  sm: { track: 'h-5 w-9', knob: 'h-4 w-4', translate: '16px' },
  md: { track: 'h-6 w-11', knob: 'h-5 w-5', translate: '20px' },
};

export function TogglePrimitive(props: BaseToggleProps): JSX.Element {
  const size = props.size ?? 'sm';
  const config = sizeConfig[size];
  const isDisabled = () => Boolean(props.disabled);
  const checkedClass = 'bg-blue-500 dark:bg-blue-500';
  const uncheckedClass = 'bg-gray-300 dark:bg-gray-600';
  const disabledClass = 'bg-gray-200 dark:bg-gray-700 cursor-not-allowed opacity-60';
  const knobBase = 'bg-white shadow-md';

  const handleClick = () => {
    if (isDisabled()) return;
    const next = !props.checked;
    let defaultPrevented = false;

    const event: ToggleChangeEvent = {
      currentTarget: { checked: next },
      preventDefault() {
        defaultPrevented = true;
      },
      stopPropagation() {
        /* noop for synthetic toggle event */
      },
      get defaultPrevented() {
        return defaultPrevented;
      },
    };

    props.onChange?.(event);
    if (!event.defaultPrevented) {
      props.onToggle?.();
    }
  };

  return (
    <button
      type="button"
      class={`relative inline-flex ${config.track} shrink-0 items-center rounded-full p-0.5 transition-colors duration-200 ease-in-out focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2 dark:focus-visible:ring-offset-gray-900 ${
        isDisabled() ? disabledClass : props.checked ? checkedClass : uncheckedClass
      } ${props.class ?? ''}`.trim()}
      onClick={handleClick}
      disabled={props.disabled}
      title={props.title}
      aria-pressed={props.checked ? 'true' : 'false'}
      aria-label={props.ariaLabel}
    >
      <span
        class={`inline-block ${config.knob} rounded-full transition-transform duration-200 ease-in-out ${knobBase} ${
          isDisabled() ? 'opacity-60' : ''
        }`}
        style={{ transform: props.checked ? `translateX(${config.translate})` : 'translateX(0)' }}
      />
    </button>
  );
}

interface LabeledToggleProps extends BaseToggleProps {
  label?: JSX.Element;
  description?: JSX.Element;
  containerClass?: string;
  locked?: boolean;
  lockedMessage?: string;
}

export function Toggle(props: LabeledToggleProps) {
  const size = props.size ?? 'md';
  return (
    <div class={`flex items-center gap-3 ${props.containerClass ?? ''}`.trim()}>
      <TogglePrimitive {...props} size={size} />
      {(props.label || props.description) && (
        <span class="flex flex-col text-sm text-gray-700 dark:text-gray-300">
          {props.label}
          {props.description && (
            <span class="text-xs text-gray-500 dark:text-gray-400">{props.description}</span>
          )}
        </span>
      )}
    </div>
  );
}

export default Toggle;
