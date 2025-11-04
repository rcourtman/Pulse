import { createSignal, onCleanup, splitProps } from 'solid-js';
import type { JSX } from 'solid-js';
import { copyToClipboard } from '@/utils/clipboard';
import { logger } from '@/utils/logger';

interface CopyButtonProps extends JSX.ButtonHTMLAttributes<HTMLButtonElement> {
  text: string;
  children: JSX.Element;
  onCopied?: () => void;
}

export function CopyButton(props: CopyButtonProps) {
  const [local, others] = splitProps(props, ['text', 'children', 'class', 'onClick', 'onCopied']);
  const [copied, setCopied] = createSignal(false);
  let resetTimeout: number | undefined;

  const handleClick = async (event: MouseEvent) => {
    const handler = local.onClick as
      | ((event: MouseEvent) => void)
      | { handleEvent?: (event: MouseEvent) => void }
      | undefined;

    if (typeof handler === 'function') {
      handler(event);
    } else if (handler && typeof handler.handleEvent === 'function') {
      handler.handleEvent(event);
    }
    if (event.defaultPrevented) {
      return;
    }

    const success = await copyToClipboard(local.text);
    if (success) {
      setCopied(true);
      window.clearTimeout(resetTimeout);
      resetTimeout = window.setTimeout(() => setCopied(false), 2000);
      if (typeof local.onCopied === 'function') {
        try {
          local.onCopied();
        } catch (error) {
          logger.error('onCopied handler failed', error);
        }
      }
    }
  };

  onCleanup(() => {
    if (resetTimeout) {
      window.clearTimeout(resetTimeout);
    }
  });

  return (
    <button
      type="button"
      {...others}
      onClick={handleClick}
      class={`inline-flex items-center justify-center gap-2 rounded-md border border-transparent bg-blue-600 px-4 py-2 text-xs sm:text-sm font-medium text-white shadow-sm transition-colors hover:bg-blue-700 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-400 ${
        local.class ?? ''
      }`}
    >
      {copied() ? (
        <>
          <svg
            class="h-4 w-4"
            viewBox="0 0 20 20"
            fill="currentColor"
            aria-hidden="true"
          >
            <path
              fill-rule="evenodd"
              d="M16.707 5.293a1 1 0 00-1.414 0L8 12.586 4.707 9.293a1 1 0 00-1.414 1.414l4 4a1 1 0 001.414 0l8-8a1 1 0 000-1.414z"
              clip-rule="evenodd"
            />
          </svg>
          <span>Copied</span>
        </>
      ) : (
        <>
          <svg
            class="h-4 w-4"
            viewBox="0 0 20 20"
            fill="currentColor"
            aria-hidden="true"
          >
            <path d="M8 2a2 2 0 00-2 2v1H5a3 3 0 00-3 3v7a3 3 0 003 3h6a3 3 0 003-3v-1h1a2 2 0 002-2V7.414a2 2 0 00-.586-1.414l-3.414-3.414A2 2 0 0013.586 2H8zm4 4V3.414L15.586 7H13a1 1 0 01-1-1z" />
            <path d="M3 8a1 1 0 011-1h6a1 1 0 011 1v7a1 1 0 01-1 1H4a1 1 0 01-1-1V8z" />
          </svg>
          <span>{local.children}</span>
        </>
      )}
    </button>
  );
}

export default CopyButton;
