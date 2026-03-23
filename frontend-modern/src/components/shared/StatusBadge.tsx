import { JSX } from 'solid-js';
import {
  getStatusBadgeClass,
  getStatusBadgeLabel,
  getStatusBadgeTitle,
  resolveStatusBadgeSize,
  type StatusBadgeProps,
} from '@/components/shared/statusBadgeModel';
import { useStatusBadgeState } from '@/components/shared/useStatusBadgeState';

export function StatusBadge(props: StatusBadgeProps): JSX.Element {
  const state = useStatusBadgeState(props);
  const size = resolveStatusBadgeSize(props.size);

  return (
    <button
      type="button"
      class={getStatusBadgeClass(size, props.isEnabled, state.isDisabled())}
      onClick={state.handleClick}
      disabled={props.disabled}
      aria-pressed={props.isEnabled}
      title={getStatusBadgeTitle(props, state.isDisabled())}
    >
      {getStatusBadgeLabel(props)}
    </button>
  );
}

export default StatusBadge;
