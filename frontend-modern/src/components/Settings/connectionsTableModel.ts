import type { Connection } from '@/api/connections';

export interface InfrastructureSystemRow {
  id: string;
  name: string;
  subtitle?: string;
  host?: string;
  coverageLabels: string[];
  statusLabel: string;
  statusClassName: string;
  lastActivityText: string;
  lastErrorMessage?: string;
  enabled: boolean;
  canEdit: boolean;
  canPause: boolean;
  canRemove: boolean;
  isAgent: boolean;
  connection: Connection;
}
