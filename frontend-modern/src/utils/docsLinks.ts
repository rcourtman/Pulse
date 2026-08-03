import { docRouteForPath } from '@/features/docs/docMarkdown';

export const SHIPPED_DOCS_ROOT = '/docs';

/**
 * URL for a shipped document. This resolves to the in-app viewer route, which
 * renders the markdown. The raw source stays served at `/docs/<name>.md` for
 * anyone who wants the file itself.
 */
export function getShippedDocUrl(filename: string): string {
  return docRouteForPath(filename);
}

export const README_DOC_URL = getShippedDocUrl('README.md');
export const MIGRATION_GUIDE_DOC_URL = getShippedDocUrl('MIGRATION_UNIFIED_NAV.md');
export const PRIVACY_DOC_URL = getShippedDocUrl('PRIVACY.md');
export const CONFIGURATION_DOC_URL = getShippedDocUrl('CONFIGURATION.md');
export const PROXY_AUTH_DOC_URL = getShippedDocUrl('PROXY_AUTH.md');
export const SECURITY_DOC_URL = getShippedDocUrl('SECURITY.md');
export const TERMS_DOC_URL = getShippedDocUrl('TERMS.md');
export const API_TOKEN_SCOPES_DOC_URL = CONFIGURATION_DOC_URL;
export const AGENT_SUBSTRATE_DOC_URL = getShippedDocUrl('AGENT_SUBSTRATE.md');
