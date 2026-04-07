export interface UpgradeDestination {
  href: string;
  external: boolean;
  hardNavigation?: boolean;
  newTab?: boolean;
  preserveOpener?: boolean;
}

export interface UpgradeDestinationOptions {
  hardNavigation?: boolean;
  newTab?: boolean;
  preserveOpener?: boolean;
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
  options: UpgradeDestinationOptions = {},
): UpgradeDestination {
  const normalized = normalizeUpgradeHref(href);
  const external = isExternalUpgradeHref(normalized);
  return {
    href: normalized,
    external,
    hardNavigation: options.hardNavigation ?? external,
    newTab: options.newTab ?? external,
    preserveOpener: options.preserveOpener ?? false,
  };
}

export function openExternalUpgradeDestination(href: string, preserveOpener = false): void {
  if (typeof window === 'undefined') return;
  if (preserveOpener) {
    window.open(href, '_blank');
    return;
  }
  window.open(href, '_blank', 'noopener,noreferrer');
}

export function navigateToUpgradeDestination(
  destination: UpgradeDestination,
  navigate: (href: string) => void,
  openExternal: (href: string, preserveOpener?: boolean) => void = openExternalUpgradeDestination,
  hardNavigate: (href: string) => void = (href) => {
    if (typeof window !== 'undefined') {
      window.location.assign(href);
    }
  },
): void {
  if (!destination.href) return;
  const newTab = destination.newTab ?? destination.external;
  const hardNavigation = destination.hardNavigation ?? destination.external;
  if (newTab) {
    openExternal(destination.href, destination.preserveOpener ?? false);
    return;
  }
  if (hardNavigation) {
    hardNavigate(destination.href);
    return;
  }
  navigate(destination.href);
}
