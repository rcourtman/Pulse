export type BackupType = 'snapshot' | 'local' | 'remote';
export type GuestType = 'VM' | 'LXC' | 'Host' | 'Template' | 'ISO';

export interface UnifiedBackup {
  source: string;
  backupType: BackupType;
  vmid: number | string;
  name: string;
  type: GuestType;
  node: string;
  instance: string; // Unique instance identifier for handling duplicate node names
  backupTime: number;
  backupName: string;
  description: string;
  status: string;
  size: number | null;
  storage: string | null;
  datastore: string | null;
  namespace: string | null;
  verified: boolean | null;
  protected: boolean;
  encrypted?: boolean;
  owner?: string;
  comment?: string; // PBS backup comment (only available from direct PBS)
}
