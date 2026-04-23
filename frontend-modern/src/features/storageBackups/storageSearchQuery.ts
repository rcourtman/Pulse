export interface ParsedStorageSearchQuery {
  freeTerms: string[];
  nodeTerms: string[];
}

export const parseStorageSearchQuery = (query: string): ParsedStorageSearchQuery => {
  const freeTerms: string[] = [];
  const nodeTerms: string[] = [];

  for (const token of query.trim().toLowerCase().split(/\s+/)) {
    if (!token) continue;
    if (token.startsWith('node:')) {
      const nodeTerm = token.slice('node:'.length).trim();
      if (nodeTerm) nodeTerms.push(nodeTerm);
      continue;
    }
    freeTerms.push(token);
  }

  return { freeTerms, nodeTerms };
};

export const matchesStorageNodeTerms = (hints: string[], nodeTerms: string[]): boolean => {
  if (nodeTerms.length === 0) return true;
  const normalizedHints = hints.map((hint) => hint.trim().toLowerCase()).filter(Boolean);
  return nodeTerms.every((term) => normalizedHints.some((hint) => hint.includes(term)));
};
