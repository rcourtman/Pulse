export type ToggleSize = 'xs' | 'sm' | 'md';

export interface ToggleChangeEvent {
  currentTarget: {
    checked: boolean;
  };
  preventDefault: () => void;
  stopPropagation: () => void;
  readonly defaultPrevented: boolean;
}

const TOGGLE_CHECKED_CLASS = 'bg-blue-500';
const TOGGLE_UNCHECKED_CLASS = 'bg-border';
const TOGGLE_DISABLED_CLASS = 'bg-base cursor-not-allowed opacity-50';
const TOGGLE_KNOB_BASE_CLASS = 'bg-surface border border-border-subtle';
const TOGGLE_CONTAINER_CLASS = 'flex items-center gap-3';
const TOGGLE_LABEL_CLASS = 'flex flex-col text-sm text-base-content';
const TOGGLE_DESCRIPTION_CLASS = 'text-xs text-muted';

const toggleSizeConfig: Record<ToggleSize, { track: string; knob: string; translateOn: string }> = {
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

export function resolveToggleSize(size: ToggleSize | undefined, fallback: ToggleSize = 'sm'): ToggleSize {
  return size ?? fallback;
}

export function getToggleTrackClass(
  size: ToggleSize,
  checked: boolean,
  disabled: boolean,
  className?: string,
): string {
  const config = toggleSizeConfig[size];
  return `relative inline-flex ${config.track} shrink-0 items-center rounded-full p-0.5 transition-all duration-300 ease-[cubic-bezier(0.34,1.56,0.64,1)] focus:outline-none focus:ring-0 ${
    disabled ? TOGGLE_DISABLED_CLASS : checked ? TOGGLE_CHECKED_CLASS : TOGGLE_UNCHECKED_CLASS
  } ${className ?? ''}`.trim();
}

export function getToggleKnobClass(size: ToggleSize, checked: boolean, disabled: boolean): string {
  const config = toggleSizeConfig[size];
  return `inline-block ${config.knob} rounded-full transition-transform duration-300 ease-[cubic-bezier(0.34,1.56,0.64,1)] ${TOGGLE_KNOB_BASE_CLASS} ${
    checked ? config.translateOn : 'translate-x-0'
  } ${disabled ? 'opacity-60' : ''}`.trim();
}

export function getToggleContainerClass(containerClass?: string): string {
  return `${TOGGLE_CONTAINER_CLASS} ${containerClass ?? ''}`.trim();
}

export function getToggleLabelClass(): string {
  return TOGGLE_LABEL_CLASS;
}

export function getToggleDescriptionClass(): string {
  return TOGGLE_DESCRIPTION_CLASS;
}
