import { getDiscoverySuggestedURLFallback } from '@/utils/discoveryPresentation';

export interface WebInterfaceUrlFieldProps {
  metadataKind: 'guest' | 'agent';
  metadataId?: string;
  targetLabel?: string;
  customUrl?: string;
  onCustomUrlChange?: (url: string) => void;
  suggestedUrl?: string;
  suggestedUrlReasonText?: string;
  suggestedUrlReasonTitle?: string;
  suggestedUrlDiagnostic?: string;
  discoveryLoading?: boolean;
  class?: string;
}

export function normalizeWebInterfaceUrl(value?: string | null): string {
  return (value || '').trim();
}

export function validateWebInterfaceCustomUrl(value: string): string | null {
  if (!value) return null;

  let parsed: URL;
  try {
    parsed = new URL(value);
  } catch {
    return 'Enter a valid URL (for example: https://198.51.100.100:8080).';
  }

  if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') {
    return 'URL must start with http:// or https://.';
  }

  if (!parsed.hostname) {
    return 'URL is missing a hostname or IP address.';
  }

  return null;
}

export function getWebInterfaceTargetLabel(
  metadataKind: WebInterfaceUrlFieldProps['metadataKind'],
  targetLabel?: string,
): string {
  const trimmed = normalizeWebInterfaceUrl(targetLabel);
  if (trimmed) return trimmed;
  return metadataKind === 'agent' ? 'agent' : 'workload';
}

export function shouldShowWebInterfaceSuggestedDiagnostic(options: {
  discoveryLoading?: boolean;
  suggestedUrl?: string;
  suggestedUrlDiagnostic?: string;
}): boolean {
  return (
    !options.discoveryLoading &&
    !normalizeWebInterfaceUrl(options.suggestedUrl) &&
    Boolean(options.suggestedUrlDiagnostic)
  );
}

export function shouldShowWebInterfaceSuggestedUrl(options: {
  currentUrl?: string;
  suggestedUrl?: string;
}): boolean {
  const suggested = normalizeWebInterfaceUrl(options.suggestedUrl);
  if (!suggested) return false;
  return suggested !== normalizeWebInterfaceUrl(options.currentUrl);
}

export function getWebInterfaceSuggestedUrlFallback(diagnostic?: string) {
  return getDiscoverySuggestedURLFallback(diagnostic);
}
