import { Component, createSignal } from 'solid-js';

/**
 * Skip-to-content link for keyboard and screen-reader users.
 *
 * It has to be the first focusable element in the document, ahead of every
 * global banner (update, security, demo), so a single Tab from the page start
 * reaches it. The shell renders it before those banners; AppLayout owns the
 * `#main` target it jumps to. Visually hidden until focused, then shown as a
 * button at the top-left.
 */
export const SkipToContentLink: Component<{ targetId?: string }> = (props) => {
  const targetId = () => props.targetId ?? 'main';
  const [focused, setFocused] = createSignal(false);

  return (
    <a
      href={`#${targetId()}`}
      onClick={() => document.getElementById(targetId())?.focus()}
      onFocus={() => setFocused(true)}
      onBlur={() => setFocused(false)}
      class={
        focused()
          ? 'absolute left-2 top-2 z-[100] rounded bg-blue-600 px-3 py-2 text-sm font-medium text-white shadow-lg outline outline-2 outline-offset-2 outline-white'
          : 'sr-only'
      }
    >
      Skip to main content
    </a>
  );
};

export default SkipToContentLink;
