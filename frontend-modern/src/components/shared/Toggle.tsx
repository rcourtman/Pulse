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

const sizeConfig: Record<ToggleSize, { track: string; knob: string; translateOn: string }> = {
  xs: {
    track: 'h-6 w-10 sm:h-5 sm:w-9',
    knob: 'h-5 w-5 sm:h-4 sm:w-4',
    translateOn: 'translate-x-4',
  },
  sm: {
    track: 'h-7 w-11 sm:h-6 sm:w-10',
    knob: 'h-6 w-6 sm:h-5 sm:w-5',
    translateOn: 'translate-x-4',
  },
  md: {
    track: 'h-8 w-12 sm:h-7 sm:w-12',
    knob: 'h-7 w-7 sm:h-6 sm:w-6',
    translateOn: 'translate-x-4 sm:translate-x-5',
  },
};

export function TogglePrimitive(props: BaseToggleProps): JSX.Element {
  const size = props.size ?? 'sm';
  const config = sizeConfig[size];
  const isDisabled = () => Boolean(props.disabled);
  const checkedClass = 'bg-blue-500';
  const uncheckedClass = 'bg-border';
  const disabledClass = 'bg-base cursor-not-allowed opacity-50';
  const knobBase = 'bg-surface border border-border-subtle';

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
      class={`relative inline-flex ${config.track} shrink-0 items-center rounded-full p-0.5 transition-all duration-300 ease-[cubic-bezier(0.34,1.56,0.64,1)] focus:outline-none focus:ring-0 ${
        isDisabled() ? disabledClass : props.checked ? checkedClass : uncheckedClass
      } ${props.class ?? ''}`.trim()}
      onClick={handleClick}
      disabled={props.disabled}
      title={props.title}
      aria-pressed={props.checked ? 'true' : 'false'}
      aria-label={props.ariaLabel}
    >
      <span
        class={`inline-block ${config.knob} rounded-full transition-transform duration-300 ease-[cubic-bezier(0.34,1.56,0.64,1)] ${knobBase} ${
          props.checked ? config.translateOn : 'translate-x-0'
        } ${isDisabled() ? 'opacity-60' : ''}`}
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
        <span class="flex flex-col text-sm text-base-content">
          {props.label}
          {props.description && <span class="text-xs text-muted">{props.description}</span>}
        </span>
      )}
    </div>
  );
}

export default Toggle;
