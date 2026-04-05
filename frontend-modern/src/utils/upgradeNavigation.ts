export interface UpgradeDestination {
  href: string;
  external: boolean;
}

function normalizeUpgradeHref(href: string | null | undefined): string {
  return href?.trim() ?? '';
}

export function isExternalUpgradeHref(href: string | null | undefined): boolean {
  const normalized = normalizeUpgradeHref(href);
  return normalized !== '' && /^https?:\/\//.test(normalized);
}

export function resolveUpgradeDestination(
  href: string | null | undefined,
): UpgradeDestination {
  const normalized = normalizeUpgradeHref(href);
  return {
    href: normalized,
    external: isExternalUpgradeHref(normalized),
  };
}

export function openExternalUpgradeDestination(href: string): void {
  if (typeof window === 'undefined') return;
  window.open(href, '_blank', 'noopener,noreferrer');
}

export function navigateToUpgradeDestination(
  destination: UpgradeDestination,
  navigate: (href: string) => void,
  openExternal: (href: string) => void = openExternalUpgradeDestination,
): void {
  if (!destination.href) return;
  if (destination.external) {
    openExternal(destination.href);
    return;
  }
  navigate(destination.href);
}
