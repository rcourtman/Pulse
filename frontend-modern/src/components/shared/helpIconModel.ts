import { getHelpContent, type HelpContent, type HelpContentId } from '@/content/help';

export interface HelpIconInlineContent {
  title: string;
  description: string;
  examples?: string[];
  docUrl?: string;
}

export interface HelpIconProps {
  contentId?: HelpContentId;
  inline?: HelpIconInlineContent;
  size?: 'xs' | 'sm' | 'md';
  class?: string;
  position?: 'top' | 'bottom';
  maxWidth?: number;
}

export interface HelpPopoverPosition {
  top: number;
  left: number;
}

const VIEWPORT_PADDING = 8;
const POPOVER_OFFSET = 8;

export const helpIconSizeClasses = {
  xs: 'w-3 h-3',
  sm: 'w-3.5 h-3.5',
  md: 'w-4 h-4',
} as const;

export function resolveHelpContent(props: HelpIconProps): HelpContent | undefined {
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
}

export function getHelpIconSize(size?: HelpIconProps['size']) {
  return size ?? 'sm';
}

export function getHelpPopoverMaxWidth(maxWidth?: number) {
  return maxWidth ?? 320;
}

export function getHelpPopoverPreferredPosition(position?: HelpIconProps['position']) {
  return position ?? 'top';
}

export function getMissingHelpContentWarning(contentId?: HelpContentId): string | undefined {
  if (!contentId) return undefined;
  return `[HelpIcon] No content found for ID: ${contentId}`;
}

export function calculateHelpPopoverPosition(options: {
  buttonRect: DOMRect;
  popoverRect: DOMRect;
  preferredPosition: 'top' | 'bottom';
  viewportWidth: number;
  viewportHeight: number;
}): HelpPopoverPosition {
  const { buttonRect, popoverRect, preferredPosition, viewportWidth, viewportHeight } = options;

  let top: number;
  let left = buttonRect.left + buttonRect.width / 2 - popoverRect.width / 2;

  if (preferredPosition === 'top') {
    top = buttonRect.top - popoverRect.height - POPOVER_OFFSET;
    if (top < VIEWPORT_PADDING) {
      top = buttonRect.bottom + POPOVER_OFFSET;
    }
  } else {
    top = buttonRect.bottom + POPOVER_OFFSET;
    if (top + popoverRect.height > viewportHeight - VIEWPORT_PADDING) {
      top = buttonRect.top - popoverRect.height - POPOVER_OFFSET;
    }
  }

  left = Math.max(
    VIEWPORT_PADDING,
    Math.min(left, viewportWidth - popoverRect.width - VIEWPORT_PADDING),
  );

  top = Math.max(
    VIEWPORT_PADDING,
    Math.min(top, viewportHeight - popoverRect.height - VIEWPORT_PADDING),
  );

  return { top, left };
}
