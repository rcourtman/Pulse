import { createMemo, type JSX } from 'solid-js';
import { StatusDot } from '@/components/shared/StatusDot';
import {
  getSimpleStatusIndicator,
  getStatusIndicatorBadgeToneClasses,
  type StatusIndicatorVariant,
} from '@/utils/status';

type StatusIndicatorBadgeSize = 'xs' | 'sm' | 'md';
type StatusIndicatorBadgeShape = 'pill' | 'rounded';

interface StatusIndicatorBadgeProps {
  status?: string | null;
  variant?: StatusIndicatorVariant;
  label?: string;
  size?: StatusIndicatorBadgeSize;
  shape?: StatusIndicatorBadgeShape;
  dot?: boolean;
  uppercase?: boolean;
  title?: string;
  class?: string;
}

const STATUS_INDICATOR_BADGE_BASE_CLASS = 'inline-flex items-center gap-1 font-medium';
const STATUS_INDICATOR_BADGE_SIZE_CLASSES: Record<StatusIndicatorBadgeSize, string> = {
  xs: 'px-2 py-0.5 text-[10px]',
  sm: 'px-2 py-0.5 text-xs',
  md: 'px-2 py-1 text-xs',
};
const STATUS_INDICATOR_BADGE_SHAPE_CLASSES: Record<StatusIndicatorBadgeShape, string> = {
  pill: 'rounded-full',
  rounded: 'rounded',
};

export function StatusIndicatorBadge(props: StatusIndicatorBadgeProps): JSX.Element {
  const indicator = createMemo(() => getSimpleStatusIndicator(props.status));
  const variant = () => props.variant ?? indicator().variant;
  const label = () => props.label ?? indicator().label;
  const size = () => props.size ?? 'sm';
  const shape = () => props.shape ?? 'pill';
  const uppercaseClass = () => (props.uppercase ? 'uppercase tracking-wide' : '');
  const className = () =>
    [
      STATUS_INDICATOR_BADGE_BASE_CLASS,
      STATUS_INDICATOR_BADGE_SHAPE_CLASSES[shape()],
      STATUS_INDICATOR_BADGE_SIZE_CLASSES[size()],
      uppercaseClass(),
      getStatusIndicatorBadgeToneClasses(variant()),
      props.class ?? '',
    ]
      .filter(Boolean)
      .join(' ');

  return (
    <span class={className()} title={props.title}>
      {props.dot ? (
        <StatusDot
          variant={variant()}
          size="xs"
          ariaHidden={true}
          class="translate-y-[0.5px]"
        />
      ) : null}
      {label()}
    </span>
  );
}

export default StatusIndicatorBadge;
