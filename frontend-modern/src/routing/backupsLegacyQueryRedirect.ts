import { buildBackupsPath } from './resourceLinks';
import { mergeRedirectQueryParams } from './navigation';

const LEGACY_QUERY_KEYS = {
  guestType: 'type',
  query: 'search',
  provider: 'source',
  backupType: 'backupType',
  group: 'group',
} as const;

type RedirectResult = {
  target: string;
  didRewrite: boolean;
};

function normalize(value: string | null | undefined): string {
  return (value || '').trim();
}

function isLegacyView(view: string): boolean {
  return view === 'artifacts' || view === 'protection';
}

function normalizeLegacyView(view: string): string {
  if (view === 'artifacts') return 'events';
  if (view === 'protection') return 'protected';
  return view;
}

function normalizeProvider(value: string): string {
  const v = normalize(value).toLowerCase();
  switch (v) {
    case 'pve':
    case 'proxmox-pve':
      return 'proxmox-pve';
    case 'pbs':
    case 'proxmox-pbs':
      return 'proxmox-pbs';
    case 'pmg':
    case 'proxmox-pmg':
      return 'proxmox-pmg';
    case 'truenas':
      return 'truenas';
    case 'kubernetes':
      return 'kubernetes';
    case 'docker':
      return 'docker';
    case 'host-agent':
      return 'host-agent';
    default:
      return v;
  }
}

function isVerificationValue(value: string): boolean {
  return value === 'verified' || value === 'unverified' || value === 'unknown';
}

function buildRedirect(search: string): RedirectResult {
  const incomingParams = new URLSearchParams(search);

  const rawView = normalize(incomingParams.get('view')).toLowerCase();
  const view = normalizeLegacyView(rawView);

  const mode = normalize(incomingParams.get('mode')) || normalize(incomingParams.get(LEGACY_QUERY_KEYS.backupType));
  const scope =
    normalize(incomingParams.get('scope')) ||
    (normalize(incomingParams.get(LEGACY_QUERY_KEYS.group)).toLowerCase() === 'guest' ? 'workload' : '');

  const rawProviderParam = normalize(incomingParams.get('provider'));
  const normalizedProviderParam = normalizeProvider(rawProviderParam);
  const legacySourceParam = normalize(incomingParams.get(LEGACY_QUERY_KEYS.provider));
  const provider = normalizeProvider(rawProviderParam || legacySourceParam);

  const query = normalize(incomingParams.get('q')) || normalize(incomingParams.get(LEGACY_QUERY_KEYS.query));

  let status = normalize(incomingParams.get('status')).toLowerCase();
  let verification = normalize(incomingParams.get('verification')).toLowerCase();

  const statusWasVerification = !verification && isVerificationValue(status);
  if (statusWasVerification) {
    verification = status;
    status = '';
  }

  const didRewrite =
    incomingParams.has(LEGACY_QUERY_KEYS.guestType) ||
    incomingParams.has(LEGACY_QUERY_KEYS.query) ||
    incomingParams.has(LEGACY_QUERY_KEYS.provider) ||
    incomingParams.has(LEGACY_QUERY_KEYS.backupType) ||
    incomingParams.has(LEGACY_QUERY_KEYS.group) ||
    isLegacyView(rawView) ||
    statusWasVerification ||
    // Also treat shorthand/legacy provider values as legacy (e.g. provider=pbs, provider=PVE).
    (rawProviderParam !== '' && normalizedProviderParam !== rawProviderParam.toLowerCase());

  const base = buildBackupsPath({
    view: view === 'events' || view === 'protected' ? (view as 'events' | 'protected') : null,
    rollupId: normalize(incomingParams.get('rollupId')) || null,
    provider: provider || null,
    cluster: normalize(incomingParams.get('cluster')) || null,
    namespace: normalize(incomingParams.get('namespace')) || null,
    mode: mode || null,
    scope: scope || null,
    status: status || null,
    verification: verification || null,
    node: normalize(incomingParams.get('node')) || null,
    query: query || null,
  });

  // Preserve any additional query params (migrated flags, debug toggles, etc.),
  // but strip the legacy keys so they do not get reintroduced after redirect.
  incomingParams.delete(LEGACY_QUERY_KEYS.guestType);
  incomingParams.delete(LEGACY_QUERY_KEYS.query);
  incomingParams.delete(LEGACY_QUERY_KEYS.provider);
  incomingParams.delete(LEGACY_QUERY_KEYS.backupType);
  incomingParams.delete(LEGACY_QUERY_KEYS.group);
  if (statusWasVerification) incomingParams.delete('status');

  const sanitizedIncoming = incomingParams.toString();
  const merged = sanitizedIncoming ? mergeRedirectQueryParams(base, `?${sanitizedIncoming}`) : base;

  return { target: merged, didRewrite };
}

// Returns a canonical /backups URL if the incoming query contains any legacy Backups params.
// This lets the rest of the app treat Backups URLs as "v6-native" only.
export function getBackupsLegacyQueryRedirectTarget(search: string): string | null {
  const { target, didRewrite } = buildRedirect(search);
  return didRewrite ? target : null;
}
