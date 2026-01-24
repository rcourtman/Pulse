import type { Component } from 'solid-js';

interface PulsePatrolLogoProps {
  class?: string;
  title?: string;
}

export const PulsePatrolLogo: Component<PulsePatrolLogoProps> = (props) => {
  const title = () => props.title ?? 'Pulse Patrol';

  return (
    <svg
      viewBox="0 0 24 24"
      xmlns="http://www.w3.org/2000/svg"
      aria-label={title()}
      class={props.class}
      fill="none"
      stroke="currentColor"
      stroke-width="2"
      stroke-linecap="round"
      stroke-linejoin="round"
    >
      <title>{title()}</title>
      {/* Shield with check */}
      <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" />
      <path d="m9 12 2 2 4-4" />
    </svg>
  );
};
