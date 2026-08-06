import { Show, type Component } from 'solid-js';

interface PulseBrandMarkProps {
  class?: string;
  decorative?: boolean;
  title?: string;
}

/**
 * Full-colour Pulse brand mark (blue disc, white ring, white centre dot).
 * Geometry and palette match docs/images/pulse-logo.svg.
 *
 * Rendered inline rather than as <img src="/logo.svg"> so the colours follow
 * the app's theme class. An <img> can only see the OS colour scheme, which
 * diverges from the app whenever pulseThemePreference is set explicitly.
 *
 * The pulse-brand-logo / pulse-bg / pulse-ring / pulse-center hooks are the
 * animation targets for .animate-pulse-brand in index.css, which runs the
 * header heartbeat while the connection is healthy.
 *
 * For the monochrome nav glyph see PulseLogoIcon.
 */
export const PulseBrandMark: Component<PulseBrandMarkProps> = (props) => {
  const title = () => props.title ?? 'Pulse Logo';
  const decorative = () => props.decorative === true;

  return (
    <svg
      viewBox="0 0 256 256"
      xmlns="http://www.w3.org/2000/svg"
      aria-hidden={decorative() ? 'true' : undefined}
      aria-label={decorative() ? undefined : title()}
      class={`pulse-brand-logo${props.class ? ` ${props.class}` : ''}`}
    >
      <Show when={!decorative()}>
        <title>{title()}</title>
      </Show>
      <circle class="pulse-bg fill-blue-600 dark:fill-blue-500" cx="128" cy="128" r="122" />
      <circle
        class="pulse-ring fill-none stroke-white dark:stroke-[#dbeafe] stroke-[14] opacity-[0.92]"
        cx="128"
        cy="128"
        r="84"
      />
      <circle class="pulse-center fill-white dark:fill-[#dbeafe]" cx="128" cy="128" r="26" />
    </svg>
  );
};
