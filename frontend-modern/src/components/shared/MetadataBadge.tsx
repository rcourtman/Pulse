import { splitProps, type Component, type JSX } from 'solid-js';

export type MetadataBadgeTone = 'neutral' | 'muted' | 'info' | 'success' | 'warning' | 'danger';
export type MetadataBadgeSize = 'xs' | 'sm' | 'md';
export type MetadataBadgeShape = 'pill' | 'rounded';

export interface MetadataBadgeProps extends Omit<JSX.HTMLAttributes<HTMLSpanElement>, 'class'> {
  tone?: MetadataBadgeTone;
  size?: MetadataBadgeSize;
  shape?: MetadataBadgeShape;
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

const METADATA_BADGE_TONE_CLASSES: Record<MetadataBadgeTone, string> = {
  neutral: 'bg-surface-alt text-base-content',
  muted: 'bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-200',
  info: 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200',
  success: 'bg-emerald-100 text-emerald-800 dark:bg-emerald-900 dark:text-emerald-200',
  warning: 'bg-amber-100 text-amber-800 dark:bg-amber-900 dark:text-amber-200',
  danger: 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200',
};

export function getMetadataBadgeClass(props: {
  tone?: MetadataBadgeTone;
  size?: MetadataBadgeSize;
  shape?: MetadataBadgeShape;
  fit?: boolean;
  uppercase?: boolean;
  class?: string;
}): string {
  const tone = props.tone ?? 'neutral';
  const size = props.size ?? 'sm';
  const shape = props.shape ?? 'pill';

  return [
    METADATA_BADGE_BASE_CLASS,
    METADATA_BADGE_SIZE_CLASSES[size],
    METADATA_BADGE_SHAPE_CLASSES[shape],
    METADATA_BADGE_TONE_CLASSES[tone],
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
