export const SHIPPED_DOCS_ROOT = '/docs';

export function getShippedDocUrl(filename: string): string {
  return `${SHIPPED_DOCS_ROOT}/${filename}`;
}

export const README_DOC_URL = getShippedDocUrl('README.md');
export const PRIVACY_DOC_URL = getShippedDocUrl('PRIVACY.md');
