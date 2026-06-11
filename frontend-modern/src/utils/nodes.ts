import type { Node } from '@/types/api';

type DisplayableNode = Pick<Node, 'name'> &
  Partial<Pick<Node, 'displayName' | 'instance' | 'host'>>;

const sanitize = (value: string): string =>
  value
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]/g, '');

const escapeRegExp = (value: string): string => value.replace(/[-/\\^$*+?.()|[\]{}]/g, '\\$&');

const extractHostname = (value: string): string => {
  if (!value) return '';
  const trimmed = value.trim();
  const withoutProtocol = trimmed.replace(/^[a-z]+:\/\//i, '');
  const [hostPart] = withoutProtocol.split('/');
  return hostPart.replace(/:\d+$/, '');
};

const HTTP_URL_PATTERN = /^https?:\/\//i;

/**
 * Resolve the external management URL for a PVE node. `guestURL` carries the
 * operator-set link override (and, via the resource adapters, the PVE API
 * connection URL). `host` is only honored when it is URL-shaped: on the
 * unified-resource node shape it holds a bare hostname label for the drawer,
 * which must not become a relative link target.
 */
export function getNodeExternalUrl<
  T extends Pick<Node, 'name'> & Partial<Pick<Node, 'guestURL' | 'host'>>,
>(node: T): string {
  const guestUrl = typeof node.guestURL === 'string' ? node.guestURL.trim() : '';
  if (HTTP_URL_PATTERN.test(guestUrl)) return guestUrl;
  const host = typeof node.host === 'string' ? node.host.trim() : '';
  if (HTTP_URL_PATTERN.test(host)) return host;
  const name = typeof node.name === 'string' ? node.name.trim() : '';
  return name ? `https://${name}:8006` : '';
}

export function getNodeDisplayName<T extends DisplayableNode>(node: T): string {
  const display = typeof node.displayName === 'string' ? node.displayName.trim() : '';
  if (display) return display;

  const nameRaw = typeof node.name === 'string' ? node.name.trim() : '';
  if (nameRaw) return nameRaw;

  const hostRaw = typeof node.host === 'string' ? node.host.trim() : '';
  const hostname = extractHostname(hostRaw);
  if (hostname) return hostname;

  const instance = typeof node.instance === 'string' ? node.instance.trim() : '';
  if (instance) return instance;

  return '';
}

export function hasAlternateDisplayName<T extends DisplayableNode>(node: T): boolean {
  const displayRaw = typeof node.displayName === 'string' ? node.displayName.trim() : '';
  if (!displayRaw) return false;

  const nameRaw = typeof node.name === 'string' ? node.name.trim() : '';
  if (!nameRaw) return false;

  const displayLower = displayRaw.toLowerCase();
  const nameLower = nameRaw.toLowerCase();

  if (displayLower === nameLower) return false;

  // Normalize values so cosmetic punctuation/domains do not trigger duplicates in the UI
  const sanitizedDisplay = sanitize(displayRaw);
  const sanitizedName = sanitize(nameRaw);

  if (sanitizedDisplay === sanitizedName) return false;

  // Catch cases where the raw display text already embeds the node name (e.g. "Friendly (node)")
  const namePattern = new RegExp(`\\b${escapeRegExp(nameLower)}\\b`, 'i');
  if (namePattern.test(displayLower)) return false;

  const [firstLabel = ''] = displayLower.split('.');
  const sanitizedFirstLabel = sanitize(firstLabel);
  if (sanitizedFirstLabel === sanitizedName) return false;

  return true;
}
