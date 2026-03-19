export const DEFAULT_ORG_SCOPE = 'default';

export const normalizeOrgScope = (orgID?: string | null): string => {
  const normalized = (orgID || '').trim();
  return normalized || DEFAULT_ORG_SCOPE;
};
