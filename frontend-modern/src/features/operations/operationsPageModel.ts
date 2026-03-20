export type OperationsTabId = 'diagnostics' | 'reporting' | 'logs';

export interface OperationsTabDefinition {
  id: OperationsTabId;
  label: string;
  description: string;
}

export const OPERATIONS_TABS: readonly OperationsTabDefinition[] = [
  {
    id: 'diagnostics',
    label: 'Diagnostics & Health',
    description: 'System health, connection tests, and troubleshooting',
  },
  {
    id: 'reporting',
    label: 'Data Export & Reports',
    description: 'Export system metrics and configuration data',
  },
  {
    id: 'logs',
    label: 'System Logs',
    description: 'View real-time Pulse system logs',
  },
];

export function getOperationsTabFromPath(pathname: string): OperationsTabId {
  const lastPathSegment = pathname.split('/').pop() || '';
  if (lastPathSegment === 'reporting') return 'reporting';
  if (lastPathSegment === 'logs') return 'logs';
  return 'diagnostics';
}

export function buildOperationsPath(tabId: OperationsTabId): string {
  return `/operations/${tabId}`;
}
