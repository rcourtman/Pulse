import { splitProps, type Component, type JSX } from 'solid-js';

export type MetadataBadgeTone =
  | 'neutral'
  | 'muted'
  | 'info'
  | 'success'
  | 'warning'
  | 'danger'
  | 'orange'
  | 'sky'
  | 'teal'
  | 'indigo';
export type MetadataBadgeSize = 'xs' | 'sm' | 'md';
export type MetadataBadgeShape = 'pill' | 'rounded';
export type MetadataBadgeAppearance = 'filled' | 'outline';

export interface MetadataBadgeProps extends Omit<JSX.HTMLAttributes<HTMLSpanElement>, 'class'> {
  tone?: MetadataBadgeTone;
  size?: MetadataBadgeSize;
  shape?: MetadataBadgeShape;
  appearance?: MetadataBadgeAppearance;
  fit?: boolean;
  uppercase?: boolean;
  class?: string;
  children?: JSX.Element;
}

const METADATA_BADGE_BASE_CLASS = 'inline-flex items-center gap-1 font-medium whitespace-nowrap';

const METADATA_BADGE_SIZE_CLASSES: Record<MetadataBadgeSize, string> = {
  xs: 'px-1.5 py-0.5 text-[10px]',
  sm: 'px-2 py-0.5 text-xs',
  md: 'px-2.5 py-1 text-xs',
};

const METADATA_BADGE_SHAPE_CLASSES: Record<MetadataBadgeShape, string> = {
  pill: 'rounded-full',
  rounded: 'rounded',
};

const METADATA_BADGE_TONE_CLASSES: Record<
  MetadataBadgeAppearance,
  Record<MetadataBadgeTone, string>
> = {
  filled: {
    neutral: 'bg-surface-alt text-base-content',
    muted: 'bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-200',
    info: 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200',
    success: 'bg-emerald-100 text-emerald-800 dark:bg-emerald-900 dark:text-emerald-200',
    warning: 'bg-amber-100 text-amber-800 dark:bg-amber-900 dark:text-amber-200',
    danger: 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200',
    orange: 'bg-orange-100 text-orange-800 dark:bg-orange-900 dark:text-orange-200',
    sky: 'bg-sky-100 text-sky-800 dark:bg-sky-900 dark:text-sky-200',
    teal: 'bg-teal-100 text-teal-800 dark:bg-teal-900 dark:text-teal-200',
    indigo: 'bg-indigo-100 text-indigo-800 dark:bg-indigo-900 dark:text-indigo-200',
  },
  outline: {
    neutral: 'border border-border bg-surface text-base-content',
    muted: 'border border-border bg-surface-alt text-muted',
    info: 'border border-blue-200 bg-blue-50 text-blue-700 dark:border-blue-800 dark:bg-blue-900 dark:text-blue-300',
    success:
      'border border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-800 dark:bg-emerald-900 dark:text-emerald-300',
    warning:
      'border border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-800 dark:bg-amber-900 dark:text-amber-300',
    danger:
      'border border-red-200 bg-red-50 text-red-700 dark:border-red-800 dark:bg-red-900 dark:text-red-300',
    orange:
      'border border-orange-200 bg-orange-50 text-orange-700 dark:border-orange-800 dark:bg-orange-900 dark:text-orange-300',
    sky: 'border border-sky-200 bg-sky-50 text-sky-700 dark:border-sky-800 dark:bg-sky-900 dark:text-sky-300',
    teal: 'border border-teal-200 bg-teal-50 text-teal-700 dark:border-teal-800 dark:bg-teal-900 dark:text-teal-300',
    indigo:
      'border border-indigo-200 bg-indigo-50 text-indigo-700 dark:border-indigo-800 dark:bg-indigo-900 dark:text-indigo-300',
  },
};

export function getMetadataBadgeClass(props: {
  tone?: MetadataBadgeTone;
  size?: MetadataBadgeSize;
  shape?: MetadataBadgeShape;
  appearance?: MetadataBadgeAppearance;
  fit?: boolean;
  uppercase?: boolean;
  class?: string;
}): string {
  const tone = props.tone ?? 'neutral';
  const size = props.size ?? 'sm';
  const shape = props.shape ?? 'pill';
  const appearance = props.appearance ?? 'filled';

  return [
    METADATA_BADGE_BASE_CLASS,
    METADATA_BADGE_SIZE_CLASSES[size],
    METADATA_BADGE_SHAPE_CLASSES[shape],
    METADATA_BADGE_TONE_CLASSES[appearance][tone],
    props.fit ? 'w-fit' : '',
    props.uppercase ? 'uppercase tracking-wide' : '',
    props.class ?? '',
  ]
    .filter(Boolean)
    .join(' ');
}

export const MetadataBadge: Component<MetadataBadgeProps> = (props) => {
  const [local, others] = splitProps(props, [
    'tone',
    'size',
    'shape',
    'appearance',
    'fit',
    'uppercase',
    'class',
    'children',
  ]);

  return (
    <span
      {...others}
      class={getMetadataBadgeClass({
        tone: local.tone,
        size: local.size,
        shape: local.shape,
        appearance: local.appearance,
        fit: local.fit,
        uppercase: local.uppercase,
        class: local.class,
      })}
    >
      {local.children}
    </span>
  );
};

export default MetadataBadge;
