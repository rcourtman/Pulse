import type { Component } from 'solid-js';
import {
  ENVIRONMENT_LOCK_BADGE_CLASS,
  ENVIRONMENT_LOCK_BADGE_LABEL,
  getEnvironmentLockTitle,
} from '@/utils/environmentLockPresentation';

interface EnvironmentLockBadgeProps {
  envVar: string;
  class?: string;
  icon?: Component<{ class?: string }>;
}

export const EnvironmentLockBadge: Component<EnvironmentLockBadgeProps> = (props) => (
  <span
    class={props.class ? `${ENVIRONMENT_LOCK_BADGE_CLASS} ${props.class}` : ENVIRONMENT_LOCK_BADGE_CLASS}
    title={getEnvironmentLockTitle(props.envVar)}
  >
    {props.icon ? <props.icon class="w-3 h-3" /> : null}
    {ENVIRONMENT_LOCK_BADGE_LABEL}
  </span>
);
