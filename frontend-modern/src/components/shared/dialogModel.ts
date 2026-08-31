export type DialogLayout = 'modal' | 'mobile-sheet' | 'drawer-right';

const FOCUSABLE_SELECTOR =
  'a[href],area[href],button:not([disabled]),input:not([disabled]):not([type="hidden"]),select:not([disabled]),textarea:not([disabled]),[tabindex]:not([tabindex="-1"])';

export function getDialogLayout(layout?: DialogLayout): DialogLayout {
  return layout ?? 'modal';
}

export function getDialogFocusableElements(container: HTMLElement): HTMLElement[] {
  return Array.from(container.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR)).filter(
    (element) =>
      !element.hasAttribute('disabled') && element.getAttribute('aria-hidden') !== 'true',
  );
}

export function getDialogViewportClass(layout: DialogLayout): string {
  const paddingClass =
    layout === 'drawer-right' ? 'p-0' : layout === 'mobile-sheet' ? 'p-3' : 'p-4 sm:p-6';
  return `relative h-full overflow-y-auto pointer-events-none ${paddingClass}`;
}

export function getDialogAlignmentClass(layout: DialogLayout): string {
  const alignmentClass =
    layout === 'drawer-right'
      ? 'items-stretch justify-end'
      : layout === 'mobile-sheet'
        ? 'items-end justify-center sm:items-center'
        : 'items-start justify-center sm:items-center';
  return `flex min-h-full ${alignmentClass}`;
}

export function getDialogPanelClass(layout: DialogLayout, panelClass?: string): string {
  const layoutClass =
    layout === 'drawer-right'
      ? 'h-dvh max-w-[720px] rounded-none border-y-0 border-r-0 animate-slide-up sm:h-full sm:max-h-dvh sm:rounded-l-xl sm:border-y sm:border-r-0'
      : layout === 'mobile-sheet'
        ? 'max-h-[calc(100dvh-1.5rem)] rounded-md animate-slide-up'
        : 'max-h-[calc(100dvh-2rem)] rounded-md animate-slide-up';
  return `relative flex min-h-0 w-full flex-col overflow-hidden bg-surface border border-border outline-none pointer-events-auto ${
    layoutClass
  } ${panelClass ?? (layout === 'drawer-right' ? '' : 'max-w-lg')}`.trim();
}
