import { Component, For, Show } from 'solid-js';
import { getTagColorWithSpecial } from '@/utils/tagColors';
import { useDarkMode } from '@/App';
import { showTooltip, hideTooltip } from '@/components/shared/Tooltip';

interface TagBadgesProps {
  tags?: string[];
  maxVisible?: number;
  isDarkMode?: boolean;
  onTagClick?: (tag: string) => void;
  activeSearch?: string;
}

export const TagBadges: Component<TagBadgesProps> = (props) => {
  // maxVisible: 0 means show all, undefined defaults to 3
  const maxVisible = () => (props.maxVisible === 0 ? Infinity : (props.maxVisible ?? 3));
  const darkModeSignal = useDarkMode();
  const isDark = () => props.isDarkMode ?? darkModeSignal();

  const visibleTags = () => props.tags?.slice(0, maxVisible()) || [];
  const hiddenTags = () => props.tags?.slice(maxVisible()) || [];

  const TagDot: Component<{ tag: string }> = (dotProps) => {
    const colors = () => getTagColorWithSpecial(dotProps.tag, isDark());
    const isActive = () => props.activeSearch?.includes(`tags:${dotProps.tag}`) || false;

    return (
      <div
        class="relative"
        onMouseEnter={(e) => {
          const rect = e.currentTarget.getBoundingClientRect();
          showTooltip(dotProps.tag, rect.left + rect.width / 2, rect.top, {
            align: 'center',
            direction: 'up',
          });
        }}
        onMouseLeave={() => {
          hideTooltip();
        }}
        onClick={(e) => {
          e.stopPropagation();
          props.onTagClick?.(dotProps.tag);
        }}
      >
        <div
          class="w-2 h-2 rounded-full hover:scale-150 transition-transform duration-200 ease-out cursor-pointer"
          style={{
            'background-color': colors().bg,
            'box-shadow': isActive()
              ? isDark()
                ? `0 0 0 2.5px rgba(255, 255, 255, 0.9)`
                : `0 0 0 2.5px rgba(0, 0, 0, 0.8)`
              : 'none',
          }}
        />
      </div>
    );
  };

  return (
    <Show when={props.tags && props.tags.length > 0}>
      <div class="inline-flex items-center gap-1 ml-2">
        <For each={visibleTags()}>{(tag) => <TagDot tag={tag} />}</For>

        {/* Show the final dot if only one hidden tag remains */}
        <Show when={hiddenTags().length === 1}>
          <TagDot tag={hiddenTags()[0]} />
        </Show>

        {/* Show +X more indicator if there are multiple hidden tags */}
        <Show when={hiddenTags().length > 1}>
          <div
            class="relative"
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
          >
            <div class="inline-flex items-center text-[10px] text-muted whitespace-nowrap leading-none cursor-pointer hover:text-base-content hover:scale-125 transition-transform duration-200 ease-out">
              +{hiddenTags().length}
            </div>
          </div>
        </Show>
      </div>
    </Show>
  );
};
