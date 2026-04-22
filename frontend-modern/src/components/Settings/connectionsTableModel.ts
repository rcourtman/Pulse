import type { Connection } from '@/api/connections';
import type { ConnectionType } from '@/api/connections';

export const connectionLastActivityText = (connection: Connection): string => {
  if (!connection.lastSeen) return 'No activity yet';
  const ts = Date.parse(connection.lastSeen);
  if (Number.isNaN(ts)) return 'Unknown';
  const diff = Math.max(0, Date.now() - ts);
  const sec = Math.floor(diff / 1000);
  if (sec < 60) return `${sec}s ago`;
  const min = Math.floor(sec / 60);
  if (min < 60) return `${min}m ago`;
  const hr = Math.floor(min / 60);
  if (hr < 24) return `${hr}h ago`;
  const days = Math.floor(hr / 24);
  return `${days}d ago`;
};

export interface InfrastructureSystemRow {
  id: string;
  ownerType: ConnectionType;
  name: string;
  subtitle?: string;
  host?: string;
  coverageLabels: string[];
  sourceBadges: string[];
  statusLabel: string;
  statusClassName: string;
  lastActivityText: string;
  lastErrorMessage?: string;
  enabled: boolean;
  canEdit: boolean;
  canPause: boolean;
  canRemove: boolean;
  isAgent: boolean;
  attachedConnections: Connection[];
  connection: Connection;
}
