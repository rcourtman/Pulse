export const SHIPPED_DOCS_ROOT = '/docs';

export function getShippedDocUrl(filename: string): string {
  return `${SHIPPED_DOCS_ROOT}/${filename}`;
}

export const README_DOC_URL = getShippedDocUrl('README.md');
export const PRIVACY_DOC_URL = getShippedDocUrl('PRIVACY.md');
export const CONFIGURATION_DOC_URL = getShippedDocUrl('CONFIGURATION.md');
export const PROXY_AUTH_DOC_URL = getShippedDocUrl('PROXY_AUTH.md');
export const SECURITY_DOC_URL = getShippedDocUrl('SECURITY.md');
export const TERMS_DOC_URL = getShippedDocUrl('TERMS.md');
export const API_TOKEN_SCOPES_DOC_URL = CONFIGURATION_DOC_URL;
