import { getResourceTypeLabel } from '@/utils/resourceTypePresentation';

const COMPOSITION_LABELS: Record<string, string> = {
  vm: 'Virtual Machines',
  'system-container': 'System Containers',
  'app-container': 'App Containers',
  pod: 'Kubernetes Pods',
  database: 'Databases',
};

const titleize = (value: string): string =>
  value
    .split(/[\s_-]+/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(' ');

export const getDashboardCompositionLabel = (resourceType: string): string => {
  const normalized = (resourceType || '').trim().toLowerCase();
  if (!normalized) return 'Unknown';
  if (COMPOSITION_LABELS[normalized]) return COMPOSITION_LABELS[normalized];
  const sharedLabel = getResourceTypeLabel(normalized);
  if (sharedLabel && sharedLabel !== normalized) return sharedLabel;
  return titleize(normalized);
};
