import Server from 'lucide-solid/icons/server';
import AppWindow from 'lucide-solid/icons/app-window';
import Database from 'lucide-solid/icons/database';
import Box from 'lucide-solid/icons/box';
import Container from 'lucide-solid/icons/container';
import { getResourceTypeLabel } from '@/utils/resourceTypePresentation';

const COMPOSITION_ICONS: Record<string, unknown> = {
  vm: Server,
  'system-container': Box,
  'app-container': Container,
  pod: AppWindow,
  database: Database,
  unknown: Server,
};

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

export const DASHBOARD_COMPOSITION_EMPTY_STATE = 'No resources detected';

export const getDashboardCompositionLabel = (resourceType: string): string => {
  const normalized = (resourceType || '').trim().toLowerCase();
  if (!normalized) return 'Unknown';
  if (COMPOSITION_LABELS[normalized]) return COMPOSITION_LABELS[normalized];
  const sharedLabel = getResourceTypeLabel(normalized);
  if (sharedLabel && sharedLabel !== normalized) return sharedLabel;
  return titleize(normalized);
};

export const getDashboardCompositionIcon = (resourceType: string): unknown => {
  const normalized = (resourceType || '').trim().toLowerCase();
  return COMPOSITION_ICONS[normalized] ?? Server;
};
