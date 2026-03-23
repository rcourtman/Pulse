import type { StatusBadgeProps } from '@/components/shared/statusBadgeModel';

export function useStatusBadgeState(props: StatusBadgeProps) {
  const isDisabled = () => Boolean(props.disabled);

  return {
    handleClick: () => {
      if (isDisabled()) {
        return;
      }
      props.onToggle?.();
    },
    isDisabled,
  };
}
