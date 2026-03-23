export interface SearchTip {
  code: string;
  description: string;
}

export interface SearchTipsPopoverProps {
  buttonLabel?: string;
  title?: string;
  intro?: string;
  tips: SearchTip[];
  footerText?: string;
  footerHighlight?: string;
  popoverId?: string;
  align?: 'left' | 'right';
  class?: string;
  triggerVariant?: 'button' | 'link' | 'icon';
  openOnHover?: boolean;
}

export function getSearchTipsPopoverPositionClass(align?: 'left' | 'right'): string {
  return align === 'left' ? 'left-0' : 'right-0';
}

export function getSearchTipsPopoverId(popoverId?: string): string {
  return popoverId ?? 'search-tips-popover';
}

export function getSearchTipsPopoverButtonLabel(buttonLabel?: string): string {
  return buttonLabel ?? 'Search tips';
}

export function getSearchTipsPopoverTitle(title?: string): string {
  return title ?? 'Search tips';
}

export function getSearchTipsPopoverTriggerVariant(
  triggerVariant?: 'button' | 'link' | 'icon',
): 'button' | 'link' | 'icon' {
  return triggerVariant ?? 'button';
}

export function shouldSearchTipsPopoverOpenOnHover(openOnHover?: boolean): boolean {
  return openOnHover ?? false;
}

export function getSearchTipsPopoverTriggerClass(
  triggerVariant: 'button' | 'link' | 'icon',
): string {
  const triggerBaseClasses =
    'text-xs font-medium focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-1 focus:ring-offset-white dark:focus:ring-blue-400';

  if (triggerVariant === 'button') {
    return `rounded-md border border-border px-2.5 py-1 text-muted transition-colors hover:bg-surface-hover ${triggerBaseClasses}`;
  }

  if (triggerVariant === 'link') {
    return `rounded px-1 py-0.5 underline decoration-dotted underline-offset-4 transition-colors hover:text-base-content ${triggerBaseClasses}`;
  }

  return `flex h-5 w-5 items-center justify-center rounded-full transition-colors hover:text-muted ${triggerBaseClasses}`;
}
