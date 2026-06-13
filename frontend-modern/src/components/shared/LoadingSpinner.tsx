import { splitProps, type JSX } from 'solid-js';

export type LoadingSpinnerSize = 'xs' | 'sm' | 'md' | 'button' | 'xl' | 'lg';
export type LoadingSpinnerTone = 'current' | 'inverse' | 'info' | 'muted';

export interface LoadingSpinnerProps extends Omit<JSX.HTMLAttributes<HTMLSpanElement>, 'class'> {
  size?: LoadingSpinnerSize;
  tone?: LoadingSpinnerTone;
  label?: string;
  class?: string;
}

const LOADING_SPINNER_SIZE_CLASSES: Record<LoadingSpinnerSize, string> = {
  xs: 'h-2 w-2 border',
  sm: 'h-3 w-3 border-2',
  md: 'h-4 w-4 border-2',
  button: 'h-5 w-5 border-2',
  xl: 'h-6 w-6 border-2',
  lg: 'h-12 w-12 border-4',
};

const LOADING_SPINNER_TONE_CLASSES: Record<LoadingSpinnerTone, string> = {
  current: 'border-current',
  inverse: 'border-white',
  info: 'border-blue-500',
  muted: 'border-slate-400',
};

export function getLoadingSpinnerClass(props: {
  size?: LoadingSpinnerSize;
  tone?: LoadingSpinnerTone;
  class?: string;
}): string {
  const size = props.size ?? 'sm';
  const tone = props.tone ?? 'current';

  return [
    'inline-block shrink-0 animate-spin rounded-full border-t-transparent',
    LOADING_SPINNER_SIZE_CLASSES[size],
    LOADING_SPINNER_TONE_CLASSES[tone],
    props.class ?? '',
  ]
    .filter(Boolean)
    .join(' ');
}

export function LoadingSpinner(props: LoadingSpinnerProps): JSX.Element {
  const [local, rest] = splitProps(props, ['size', 'tone', 'label', 'class']);
  const ariaHidden = () => (local.label ? undefined : 'true');

  return (
    <span
      {...rest}
      class={getLoadingSpinnerClass({
        size: local.size,
        tone: local.tone,
        class: local.class,
      })}
      role={local.label ? 'status' : undefined}
      aria-label={local.label}
      aria-hidden={ariaHidden()}
    />
  );
}

export default LoadingSpinner;
