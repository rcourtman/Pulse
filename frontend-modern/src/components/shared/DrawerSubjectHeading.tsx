import type { Component, JSX } from 'solid-js';
import { StatusDot } from '@/components/shared/StatusDot';
import type { StatusIndicatorVariant } from '@/utils/status';

interface DrawerSubjectHeadingProps {
  headingId: string;
  title: string;
  statusVariant: StatusIndicatorVariant;
  statusLabel: string;
  trailing?: JSX.Element;
}

export const DrawerSubjectHeading: Component<DrawerSubjectHeadingProps> = (props) => (
  <div class="flex items-center gap-2 min-w-0">
    <StatusDot
      size="sm"
      variant={props.statusVariant}
      title={props.statusLabel}
      ariaLabel={props.statusLabel}
    />
    <h2
      id={props.headingId}
      class="text-sm font-semibold text-base-content truncate m-0"
      title={props.title}
    >
      {props.title}
    </h2>
    {props.trailing}
  </div>
);
