function sortedEntries(params: URLSearchParams): Array<[string, string]> {
  const entries = Array.from(params.entries());
  entries.sort((a, b) => {
    const keyCmp = a[0].localeCompare(b[0]);
    if (keyCmp !== 0) return keyCmp;
    return a[1].localeCompare(b[1]);
  });
  return entries;
}

/**
 * Compare search params by key/value pairs, ignoring ordering.
 * This prevents redirect loops when the router canonicalizes query ordering.
 */
export function areSearchParamsEquivalent(a: URLSearchParams, b: URLSearchParams): boolean {
  const ea = sortedEntries(a);
  const eb = sortedEntries(b);
  if (ea.length !== eb.length) return false;
  for (let i = 0; i < ea.length; i += 1) {
    if (ea[i][0] !== eb[i][0] || ea[i][1] !== eb[i][1]) {
      return false;
    }
  }
  return true;
}

