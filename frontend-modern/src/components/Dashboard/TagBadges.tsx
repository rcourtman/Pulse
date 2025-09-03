import { Component, For, Show, createSignal } from 'solid-js';
import { Portal } from 'solid-js/web';
import { getTagColorWithSpecial } from '@/utils/tagColors';
import { useDarkMode } from '@/App';

interface TagBadgesProps {
  tags: string[];
  maxVisible?: number;
  isDarkMode?: boolean;
  onTagClick?: (tag: string) => void;
  activeSearch?: string;
}

export const TagBadges: Component<TagBadgesProps> = (props) => {
  const maxVisible = () => props.maxVisible ?? 3;
  const darkModeSignal = useDarkMode();
  const isDark = () => props.isDarkMode ?? darkModeSignal();
  
  const visibleTags = () => props.tags?.slice(0, maxVisible()) || [];
  const hiddenTags = () => props.tags?.slice(maxVisible()) || [];
  const hasHiddenTags = () => hiddenTags().length > 0;
  
  const [hoveredTag, setHoveredTag] = createSignal<string | null>(null);
  const [tooltipPos, setTooltipPos] = createSignal<{ x: number; y: number } | null>(null);
  
  return (
    <Show when={props.tags && props.tags.length > 0}>
      <div class="inline-flex items-center gap-1 ml-2">
        <For each={visibleTags()}>
          {(tag) => {
            const colors = () => getTagColorWithSpecial(tag, isDark());
            const isActive = () => props.activeSearch?.includes(`tags:${tag}`) || false;
            
            return (
              <div
                class="relative group"
                onMouseEnter={(e) => {
                  setHoveredTag(tag);
                  const rect = e.currentTarget.getBoundingClientRect();
                  setTooltipPos({ x: rect.left + rect.width / 2, y: rect.top });
                }}
                onMouseLeave={() => {
                  setHoveredTag(null);
                  setTooltipPos(null);
                }}
                onClick={(e) => {
                  e.stopPropagation();
                  props.onTagClick?.(tag);
                }}
              >
                {/* Colored dot indicator */}
                <div 
                  class="w-2 h-2 rounded-full hover:scale-150 transition-transform duration-200 ease-out cursor-pointer"
                  style={{
                    'background-color': colors().bg,
                    'box-shadow': isActive() 
                      ? isDark() 
                        ? `0 0 0 2.5px rgba(255, 255, 255, 0.9)` // White ring in dark mode when active
                        : `0 0 0 2.5px rgba(0, 0, 0, 0.8)` // Black ring in light mode when active
                      : 'none', // No box-shadow when not active - just flat circle
                  }}
                />
              </div>
            );
          }}
        </For>
        
        {/* Show +X more indicator if there are hidden tags */}
        <Show when={hasHiddenTags()}>
          <div 
            class="relative group"
            onMouseEnter={(e) => {
              setHoveredTag('more');
              const rect = e.currentTarget.getBoundingClientRect();
              setTooltipPos({ x: rect.left + rect.width / 2, y: rect.top });
            }}
            onMouseLeave={() => {
              setHoveredTag(null);
              setTooltipPos(null);
            }}
          >
            <div class="text-[10px] text-gray-500 dark:text-gray-400 cursor-pointer hover:text-gray-700 dark:hover:text-gray-300 hover:scale-125 transition-transform duration-200 ease-out">
              +{hiddenTags().length}
            </div>
          </div>
        </Show>
      </div>
      
      {/* Render tooltips in a portal to avoid z-index issues */}
      <Portal>
        <Show when={hoveredTag() && tooltipPos()}>
          {hoveredTag() === 'more' ? (
            // Tooltip for hidden tags
            <div 
              class="fixed px-2 py-1 bg-gray-800 dark:bg-gray-700 text-white text-xs rounded shadow-lg pointer-events-none"
              style={{
                left: `${tooltipPos()!.x}px`,
                top: `${tooltipPos()!.y - 40}px`,
                transform: 'translateX(-50%)',
                'z-index': '999999',
              }}
            >
              <div class="space-y-0.5">
                <For each={hiddenTags()}>
                  {(tag) => <div>{tag}</div>}
                </For>
              </div>
            </div>
          ) : (
            // Tooltip for individual tag
            (() => {
              const tag = hoveredTag()!;
              const colors = () => getTagColorWithSpecial(tag, isDark());
              return (
                <div 
                  class="fixed px-2 py-1 text-xs rounded shadow-lg pointer-events-none"
                  style={{
                    left: `${tooltipPos()!.x}px`,
                    top: `${tooltipPos()!.y - 35}px`,
                    transform: 'translateX(-50%)',
                    'background-color': colors().bg,
                    'color': colors().text,
                    'border': `1px solid ${colors().border}`,
                    'z-index': '999999',
                  }}
                >
                  {tag}
                </div>
              );
            })()
          )}
        </Show>
      </Portal>
    </Show>
  );
};