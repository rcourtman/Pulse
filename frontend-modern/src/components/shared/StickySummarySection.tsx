import type { Component, JSX } from 'solid-js';

interface StickySummarySectionProps {
  children: JSX.Element;
  class?: string;
  desktopOnly?: boolean;
}

export const StickySummarySection: Component<StickySummarySectionProps> = (props) => {
  const desktopOnly = () => props.desktopOnly !== false;

  return (
    <div
      data-sticky-summary="true"
      data-sticky-summary-desktop-only={desktopOnly() ? 'true' : 'false'}
      class={`${desktopOnly() ? 'hidden lg:block ' : ''}sticky-shield sticky top-0 z-20 bg-surface ${
        props.class ?? ''
      }`.trim()}
    >
      {props.children}
    </div>
  );
};

export default StickySummarySection;
