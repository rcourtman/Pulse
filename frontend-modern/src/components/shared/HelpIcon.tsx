import { Component, Show } from 'solid-js';
import { Portal } from 'solid-js/web';
import CircleHelp from 'lucide-solid/icons/circle-help';
import ExternalLink from 'lucide-solid/icons/external-link';
import X from 'lucide-solid/icons/x';
import { helpIconSizeClasses, type HelpIconProps } from './helpIconModel';
import { useHelpIconState } from './useHelpIconState';

export type { HelpIconProps } from './helpIconModel';

export const HelpIcon: Component<HelpIconProps> = (props) => {
  const state = useHelpIconState(props);
  const helpContent = state.helpContent();

  // Don't render if no content available
  if (!helpContent) {
    return null;
  }

  return (
    <>
      <button
        ref={state.setButtonRef}
        type="button"
        class={`inline-flex min-h-11 min-w-11 items-center justify-center rounded-full p-1 text-slate-400 transition-colors hover:text-blue-500 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 sm:min-h-8 sm:min-w-8 dark:hover:text-blue-400 ${props.class ?? ''}`}
        onClick={state.toggleOpen}
        aria-label={`Help: ${helpContent.title}`}
        aria-expanded={state.isOpen()}
        aria-haspopup="dialog"
      >
        <CircleHelp class={helpIconSizeClasses[state.size()]} strokeWidth={2} />
      </button>

      <Show when={state.isOpen()}>
        <Portal mount={document.body}>
          <div
            ref={state.setPopoverRef}
            role="dialog"
            aria-labelledby="help-popover-title"
            class="fixed z-[9999] bg-surface rounded-md shadow-sm border border-border overflow-hidden animate-in fade-in-0 zoom-in-95 duration-150"
            style={{
              top: `${state.popoverPosition().top}px`,
              left: `${state.popoverPosition().left}px`,
              'max-width': `${state.maxWidth()}px`,
              'min-width': '200px',
            }}
          >
            {/* Header */}
            <div class="px-3 py-2 bg-surface-alt border-b border-border flex items-center justify-between gap-2">
              <span id="help-popover-title" class="text-sm font-medium text-base-content">
                {helpContent.title}
              </span>
              <button
                type="button"
                class="p-0.5 text-slate-400 hover:text-muted rounded transition-colors"
                onClick={() => state.setIsOpen(false)}
                aria-label="Close help"
              >
                <X class="w-3.5 h-3.5" strokeWidth={2} />
              </button>
            </div>

            {/* Content */}
            <div class="px-3 py-2.5 text-xs text-muted space-y-2">
              <p class="whitespace-pre-line leading-relaxed">{helpContent.description}</p>

              <Show when={helpContent.examples && helpContent.examples.length > 0}>
                <div class="pt-2 border-t border-border-subtle">
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
                <div class="pt-2 border-t border-border-subtle">
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
