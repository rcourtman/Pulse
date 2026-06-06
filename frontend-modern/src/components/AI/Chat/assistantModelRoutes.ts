const MODEL_ROUTE_PROVIDER_RE = /^[a-z][a-z0-9_-]*$/i;

export const isAssistantExplicitModelRoute = (modelId: string) => {
  const candidate = modelId.trim();
  if (!candidate || /\s/.test(candidate) || candidate.includes('://')) {
    return false;
  }
  const separator = candidate.indexOf(':');
  if (separator <= 0 || separator === candidate.length - 1) {
    return false;
  }
  const provider = candidate.slice(0, separator);
  const model = candidate.slice(separator + 1);
  return MODEL_ROUTE_PROVIDER_RE.test(provider) && Boolean(model.trim()) && !model.startsWith('/');
};

export const normalizeAssistantRecentModelRoutes = (values: unknown[], limit: number): string[] => {
  const seen = new Set<string>();
  const routes: string[] = [];

  for (const value of values) {
    const modelId = typeof value === 'string' ? value.trim() : '';
    if (!isAssistantExplicitModelRoute(modelId) || seen.has(modelId)) continue;
    seen.add(modelId);
    routes.push(modelId);
    if (routes.length >= limit) break;
  }

  return routes;
};

export const getNextAssistantRecentModelRoute = (args: {
  currentModel?: string;
  direction?: 1 | -1;
  recentModelIds: string[];
}): string | null => {
  const routes = normalizeAssistantRecentModelRoutes(
    args.recentModelIds,
    args.recentModelIds.length,
  );
  if (routes.length === 0) return null;

  const direction = args.direction ?? 1;
  const current = args.currentModel?.trim() || '';
  const currentIndex = routes.indexOf(current);
  if (currentIndex < 0) {
    return direction === 1 ? routes[0] : routes[routes.length - 1];
  }
  if (routes.length <= 1) return null;

  const nextIndex = (currentIndex + direction + routes.length) % routes.length;
  return routes[nextIndex] || null;
};
