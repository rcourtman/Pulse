import { createMemo, type JSX } from 'solid-js';
import { StatusDot } from '@/components/shared/StatusDot';
import { StatusIndicatorBadge } from '@/components/shared/StatusIndicatorBadge';
import { getAlertSeverityIndicator } from '@/utils/alertSeverityPresentation';

type AlertSeverityIndicatorSize = 'xs' | 'sm' | 'md';
type AlertSeverityBadgeShape = 'pill' | 'rounded';

interface AlertSeverityPresentationProps {
  severity?: string | null;
  bucket?: string | null;
  title?: string;
  class?: string;
}

interface AlertSeverityBadgeProps extends AlertSeverityPresentationProps {
  size?: AlertSeverityIndicatorSize;
  shape?: AlertSeverityBadgeShape;
  dot?: boolean;
  uppercase?: boolean;
}

interface AlertSeverityDotProps extends AlertSeverityPresentationProps {
  size?: AlertSeverityIndicatorSize;
  ariaLabel?: string;
  ariaHidden?: boolean;
}

export function AlertSeverityBadge(props: AlertSeverityBadgeProps): JSX.Element {
  const indicator = createMemo(() => getAlertSeverityIndicator(props.severity, props.bucket));

  return (
    <StatusIndicatorBadge
      variant={indicator().variant}
      label={indicator().label}
      size={props.size ?? 'xs'}
      shape={props.shape ?? 'pill'}
      dot={props.dot}
      uppercase={props.uppercase}
      title={props.title}
      class={props.class}
    />
  );
}

export function AlertSeverityDot(props: AlertSeverityDotProps): JSX.Element {
  const indicator = createMemo(() => getAlertSeverityIndicator(props.severity, props.bucket));

  return (
    <StatusDot
      variant={indicator().variant}
      size={props.size ?? 'sm'}
      title={props.title ?? indicator().label}
      ariaLabel={props.ariaLabel}
      ariaHidden={props.ariaHidden}
      class={props.class}
    />
  );
}
