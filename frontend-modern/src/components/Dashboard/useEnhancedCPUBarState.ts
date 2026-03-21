import { createMemo } from 'solid-js';
import { useTooltip } from '@/hooks/useTooltip';
import {
  buildEnhancedCPUBarPresentation,
  type EnhancedCPUBarProps,
} from './enhancedCpuBarModel';

export function useEnhancedCPUBarState(props: EnhancedCPUBarProps) {
  const tip = useTooltip();
  const presentation = createMemo(() => buildEnhancedCPUBarPresentation(props));

  return {
    handleMouseEnter: tip.onMouseEnter,
    handleMouseLeave: tip.onMouseLeave,
    presentation,
    tip,
    tooltipVisible: createMemo(() => tip.show()),
  };
}

