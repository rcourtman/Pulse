import type { Component, JSX } from 'solid-js';

interface StickySummarySectionProps {
  children: JSX.Element;
  class?: string;
  desktopOnly?: boolean;
  stickyDesktopOnly?: boolean;
}

export const StickySummarySection: Component<StickySummarySectionProps> = (props) => {
  const desktopOnly = () => props.desktopOnly !== false;
  const stickyDesktopOnly = () => props.stickyDesktopOnly === true;
  const stickyClass = () => (stickyDesktopOnly() ? 'static lg:sticky' : 'sticky');

  return (
    <div
      data-sticky-summary="true"
      data-sticky-summary-desktop-only={desktopOnly() ? 'true' : 'false'}
      data-sticky-summary-sticky-desktop-only={stickyDesktopOnly() ? 'true' : 'false'}
      class={`${desktopOnly() ? 'hidden lg:block ' : ''}sticky-shield ${stickyClass()} top-0 z-20 bg-surface ${
        props.class ?? ''
      }`.trim()}
    >
      {props.children}
    </div>
  );
};

export default StickySummarySection;
