import { Component, Show, createSignal, createEffect, onCleanup } from 'solid-js';
import { Portal } from 'solid-js/web';
import CircleHelp from 'lucide-solid/icons/circle-help';
import ExternalLink from 'lucide-solid/icons/external-link';
import X from 'lucide-solid/icons/x';
import { getHelpContent, type HelpContentId, type HelpContent } from '@/content/help';

export interface HelpIconProps {
  /** Help content ID from registry */
  contentId?: HelpContentId;

  /** Inline content (alternative to contentId for one-off help) */
  inline?: {
    title: string;
    description: string;
    examples?: string[];
    docUrl?: string;
  };

  /** Icon size variant */
  size?: 'xs' | 'sm' | 'md';

  /** Additional CSS classes for the button */
  class?: string;

  /** Popover position relative to icon */
  position?: 'top' | 'bottom';

  /** Max width of popover in pixels */
  maxWidth?: number;
}

const sizeClasses = {
  xs: 'w-3 h-3',
  sm: 'w-3.5 h-3.5',
  md: 'w-4 h-4',
};

export const HelpIcon: Component<HelpIconProps> = (props) => {
  const [isOpen, setIsOpen] = createSignal(false);
  const [popoverPosition, setPopoverPosition] = createSignal({ top: 0, left: 0 });
  let buttonRef: HTMLButtonElement | undefined;
  let popoverRef: HTMLDivElement | undefined;

  // Get content from registry or inline prop
  const content = (): HelpContent | undefined => {
    if (props.inline) {
      return {
        id: 'inline',
        title: props.inline.title,
        description: props.inline.description,
        examples: props.inline.examples,
        docUrl: props.inline.docUrl,
      };
    }
    if (props.contentId) {
      return getHelpContent(props.contentId);
    }
    return undefined;
  };

  const size = () => props.size ?? 'sm';
  const maxWidth = () => props.maxWidth ?? 320;
  const preferredPosition = () => props.position ?? 'top';

  // Calculate popover position when opened
  createEffect(() => {
    if (!isOpen() || !buttonRef) return;

    // Use requestAnimationFrame to ensure DOM is ready
    requestAnimationFrame(() => {
      if (!buttonRef || !popoverRef) return;

      const buttonRect = buttonRef.getBoundingClientRect();
      const popoverRect = popoverRef.getBoundingClientRect();
      const viewportPadding = 8;

      let top: number;
      let left = buttonRect.left + buttonRect.width / 2 - popoverRect.width / 2;

      // Position above or below based on preference and available space
      if (preferredPosition() === 'top') {
        top = buttonRect.top - popoverRect.height - 8;
        // If not enough space above, flip to below
        if (top < viewportPadding) {
          top = buttonRect.bottom + 8;
        }
      } else {
        top = buttonRect.bottom + 8;
        // If not enough space below, flip to above
        if (top + popoverRect.height > window.innerHeight - viewportPadding) {
          top = buttonRect.top - popoverRect.height - 8;
        }
      }

      // Clamp horizontal position to viewport
      left = Math.max(viewportPadding, Math.min(left, window.innerWidth - popoverRect.width - viewportPadding));

      // Clamp vertical position to viewport
      top = Math.max(viewportPadding, Math.min(top, window.innerHeight - popoverRect.height - viewportPadding));

      setPopoverPosition({ top, left });
    });
  });

  // Close on outside click
  createEffect(() => {
    if (!isOpen()) return;

    const handleClickOutside = (e: MouseEvent) => {
      const target = e.target as Node;
      if (buttonRef?.contains(target) || popoverRef?.contains(target)) return;
      setIsOpen(false);
    };

    const handleEscape = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        setIsOpen(false);
        buttonRef?.focus();
      }
    };

    // Delay adding listeners to avoid immediate close from the opening click
    const timeoutId = setTimeout(() => {
      document.addEventListener('click', handleClickOutside);
      document.addEventListener('keydown', handleEscape);
    }, 0);

    onCleanup(() => {
      clearTimeout(timeoutId);
      document.removeEventListener('click', handleClickOutside);
      document.removeEventListener('keydown', handleEscape);
    });
  });

  const helpContent = content();

  // Don't render if no content available
  if (!helpContent) {
    if (props.contentId) {
      console.warn(`[HelpIcon] No content found for ID: ${props.contentId}`);
    }
    return null;
  }

  return (
    <>
      <button
        ref={buttonRef}
        type="button"
        class={`inline-flex min-h-11 min-w-11 items-center justify-center rounded-full p-1 text-slate-400 transition-colors hover:text-blue-500 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 sm:min-h-8 sm:min-w-8 dark:text-slate-500 dark:hover:text-blue-400 ${props.class ?? ''}`}
        onClick={(e) => {
          e.stopPropagation();
          setIsOpen(!isOpen());
        }}
        aria-label={`Help: ${helpContent.title}`}
        aria-expanded={isOpen()}
        aria-haspopup="dialog"
      >
        <CircleHelp class={sizeClasses[size()]} strokeWidth={2} />
      </button>

      <Show when={isOpen()}>
        <Portal mount={document.body}>
          <div
            ref={popoverRef}
            role="dialog"
            aria-labelledby="help-popover-title"
            class="fixed z-[9999] bg-surface rounded-md shadow-sm border border-border overflow-hidden animate-in fade-in-0 zoom-in-95 duration-150"
            style={{
              top: `${popoverPosition().top}px`,
              left: `${popoverPosition().left}px`,
              'max-width': `${maxWidth()}px`,
              'min-width': '200px',
            }}
          >
            {/* Header */}
            <div class="px-3 py-2 bg-slate-50 dark:bg-slate-800 border-b border-border flex items-center justify-between gap-2">
              <span id="help-popover-title" class="text-sm font-medium text-base-content">
                {helpContent.title}
              </span>
              <button
                type="button"
                class="p-0.5 text-slate-400 hover:text-slate-600 dark:hover:text-slate-300 rounded transition-colors"
                onClick={() => setIsOpen(false)}
                aria-label="Close help"
              >
                <X class="w-3.5 h-3.5" strokeWidth={2} />
              </button>
            </div>

            {/* Content */}
            <div class="px-3 py-2.5 text-xs text-slate-600 dark:text-slate-300 space-y-2">
              <p class="whitespace-pre-line leading-relaxed">{helpContent.description}</p>

              <Show when={helpContent.examples && helpContent.examples.length > 0}>
                <div class="pt-2 border-t border-slate-100 dark:border-slate-700">
                  <p class="text-[10px] uppercase tracking-wide text-muted font-medium mb-1.5">
                    Examples
                  </p>
                  <ul class="space-y-1 text-[11px]">
                    {helpContent.examples!.map((example) => (
                      <li class="flex items-start gap-1.5">
                        <span class="text-muted mt-0.5 select-none">-</span>
                        <span class="text-muted">{example}</span>
                      </li>
                    ))}
                  </ul>
                </div>
              </Show>

              <Show when={helpContent.docUrl}>
                <div class="pt-2 border-t border-slate-100 dark:border-slate-700">
                  <a
                    href={helpContent.docUrl}
                    target="_blank"
                    rel="noopener noreferrer"
                    class="inline-flex items-center gap-1 text-[11px] font-medium text-blue-600 dark:text-blue-400 hover:text-blue-700 dark:hover:text-blue-300 hover:underline"
                  >
                    Learn more
                    <ExternalLink class="w-3 h-3" strokeWidth={2} />
                  </a>
                </div>
              </Show>
            </div>
          </div>
        </Portal>
      </Show>
    </>
  );
};

export default HelpIcon;
