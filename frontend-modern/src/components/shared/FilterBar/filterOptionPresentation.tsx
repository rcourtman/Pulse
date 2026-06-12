import type { JSX } from 'solid-js';

export const filterChipStatusDot = (className: string): JSX.Element => (
  <span class={`h-2 w-2 rounded-full ${className}`} aria-hidden="true" />
);
