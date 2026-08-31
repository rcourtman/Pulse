import { Component, For, Show } from 'solid-js';
import { getTagColorWithSpecial } from '@/utils/tagColors';
import { useDarkMode } from '@/contexts/appRuntime';
import { showTooltip, hideTooltip } from '@/components/shared/Tooltip';
import { getGlobalWebSocketStore } from '@/stores/websocket-global';
import type { PVETagStyle } from '@/types/api';

interface TagBadgesProps {
  tags?: string[];
  maxVisible?: number;
  isDarkMode?: boolean;
  sourceInstance?: string;
  pveTagStyle?: PVETagStyle;
  onTagClick?: (tag: string) => void;
  activeSearch?: string;
}

export const TagBadges: Component<TagBadgesProps> = (props) => {
  // maxVisible: 0 means show all, undefined defaults to 3
  const maxVisible = () => (props.maxVisible === 0 ? Infinity : (props.maxVisible ?? 3));
  const darkModeSignal = useDarkMode();
  const isDark = () => props.isDarkMode ?? darkModeSignal();
  const ws = getGlobalWebSocketStore();
  const instanceTagStyle = () =>
    props.sourceInstance ? ws.state.pveTagStyles?.[props.sourceInstance] : undefined;
  const pveTagStyle = () => props.pveTagStyle ?? instanceTagStyle();
  const pveTagColors = () => pveTagStyle()?.colors ?? ws.state.pveTagColors;
  const pveTagCaseSensitive = () => pveTagStyle()?.caseSensitive ?? false;

  const visibleTags = () => props.tags?.slice(0, maxVisible()) || [];
  const hiddenTags = () => props.tags?.slice(maxVisible()) || [];

  const TagDot: Component<{ tag: string }> = (dotProps) => {
    const colors = () =>
      getTagColorWithSpecial(dotProps.tag, isDark(), pveTagColors(), {
        caseSensitive: pveTagCaseSensitive(),
      });
    const isActive = () => props.activeSearch?.includes(`tags:${dotProps.tag}`) || false;
    const ringClass = () =>
      isActive() ? (isDark() ? 'text-white/90' : 'text-black/80') : 'text-transparent';

    const showTagTooltip = (element: HTMLElement) => {
      const rect = element.getBoundingClientRect();
      showTooltip(dotProps.tag, rect.left + rect.width / 2, rect.top, {
        align: 'center',
        direction: 'up',
      });
    };

    const dot = () => (
      <svg
        data-tag-dot="true"
        data-active={isActive() ? 'true' : 'false'}
        viewBox="0 0 10 10"
        aria-hidden="true"
        class={`h-2 w-2 overflow-visible transition-transform duration-200 ease-out group-hover/tag:scale-150 group-focus-visible/tag:scale-150 ${ringClass()}`}
      >
        <circle
          data-tag-dot-ring="true"
          cx="5"
          cy="5"
          r="4"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
        />
        <circle data-tag-dot-fill="true" cx="5" cy="5" r="3" fill={colors().bg} />
      </svg>
    );

    const sharedClass =
      'group/tag relative inline-flex h-5 w-5 shrink-0 items-center justify-center rounded-full focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sky-500 focus-visible:ring-offset-1 focus-visible:ring-offset-surface';

    return (
      <Show
        when={props.onTagClick}
        fallback={
          <span
            class={`${sharedClass} cursor-help`}
            role="img"
            aria-label={`Tag: ${dotProps.tag}`}
            tabIndex={0}
            title={dotProps.tag}
            onMouseEnter={(event) => showTagTooltip(event.currentTarget)}
            onMouseLeave={hideTooltip}
            onFocus={(event) => showTagTooltip(event.currentTarget)}
            onBlur={hideTooltip}
            onKeyDown={(event) => {
              if (event.key === 'Escape') hideTooltip();
            }}
          >
            {dot()}
          </span>
        }
      >
        {(onTagClick) => (
          <button
            type="button"
            class={`${sharedClass} cursor-pointer`}
            aria-label={`Filter by tag ${dotProps.tag}`}
            aria-pressed={isActive()}
            title={dotProps.tag}
            onMouseEnter={(event) => showTagTooltip(event.currentTarget)}
            onMouseLeave={hideTooltip}
            onFocus={(event) => showTagTooltip(event.currentTarget)}
            onBlur={hideTooltip}
            onKeyDown={(event) => {
              if (event.key === 'Escape') hideTooltip();
            }}
            onClick={(event) => {
              event.stopPropagation();
              onTagClick()(dotProps.tag);
            }}
          >
            {dot()}
          </button>
        )}
      </Show>
    );
  };

  return (
    <Show when={props.tags && props.tags.length > 0}>
      <div class="ml-2 inline-flex items-center gap-1" role="group" aria-label="Tags">
        <For each={visibleTags()}>{(tag) => <TagDot tag={tag} />}</For>

        {/* Show the final dot if only one hidden tag remains */}
        <Show when={hiddenTags().length === 1}>
          <TagDot tag={hiddenTags()[0]} />
        </Show>

        {/* Show +X more indicator if there are multiple hidden tags */}
        <Show when={hiddenTags().length > 1}>
          <span
            class="relative inline-flex h-5 min-w-5 cursor-help items-center justify-center rounded px-1 text-[10px] leading-none text-muted transition-colors hover:text-base-content focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sky-500 focus-visible:ring-offset-1 focus-visible:ring-offset-surface"
            role="img"
            aria-label={`${hiddenTags().length} more tags: ${hiddenTags().join(', ')}`}
            tabIndex={0}
            title={hiddenTags().join(', ')}
            onMouseEnter={(e) => {
              const rect = e.currentTarget.getBoundingClientRect();
              const content = hiddenTags().join('\n');
              if (content) {
                showTooltip(content, rect.left + rect.width / 2, rect.top, {
                  align: 'center',
                  direction: 'up',
                  maxWidth: 260,
                });
              }
            }}
            onMouseLeave={() => {
              hideTooltip();
            }}
            onFocus={(e) => {
              const rect = e.currentTarget.getBoundingClientRect();
              const content = hiddenTags().join('\n');
              if (content) {
                showTooltip(content, rect.left + rect.width / 2, rect.top, {
                  align: 'center',
                  direction: 'up',
                  maxWidth: 260,
                });
              }
            }}
            onBlur={hideTooltip}
            onKeyDown={(event) => {
              if (event.key === 'Escape') hideTooltip();
            }}
          >
            <span aria-hidden="true" class="whitespace-nowrap">
              +{hiddenTags().length}
            </span>
          </span>
        </Show>
      </div>
    </Show>
  );
};
