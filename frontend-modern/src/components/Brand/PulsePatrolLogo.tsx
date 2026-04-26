import { Show, type Component } from 'solid-js';

interface PulsePatrolLogoProps {
  class?: string;
  decorative?: boolean;
  title?: string;
}

export const PulsePatrolLogo: Component<PulsePatrolLogoProps> = (props) => {
  const title = () => props.title ?? 'Pulse Patrol';
  const decorative = () => props.decorative === true;

  return (
    <svg
      viewBox="0 0 24 24"
      xmlns="http://www.w3.org/2000/svg"
      aria-hidden={decorative() ? 'true' : undefined}
      aria-label={decorative() ? undefined : title()}
      class={props.class}
      fill="none"
      stroke="currentColor"
      stroke-width="2"
      stroke-linecap="round"
      stroke-linejoin="round"
    >
      <Show when={!decorative()}>
        <title>{title()}</title>
      </Show>
      {/* Infinity loop */}
      <path d="M12 12c-2-2.67-4-4-6-4a4 4 0 1 0 0 8c2 0 4-1.33 6-4Zm0 0c2 2.67 4 4 6 4a4 4 0 1 0 0-8c-2 0-4 1.33-6 4Z" />
    </svg>
  );
};
