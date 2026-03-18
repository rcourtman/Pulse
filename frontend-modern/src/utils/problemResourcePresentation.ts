import type { StatusIndicatorVariant } from '@/utils/status';

export function getProblemResourceStatusVariant(worstValue: number): StatusIndicatorVariant {
  return worstValue >= 150 ? 'danger' : 'warning';
}
