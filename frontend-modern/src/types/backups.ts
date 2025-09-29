export type BackupType = 'snapshot' | 'local' | 'remote';
export type GuestType = 'VM' | 'LXC' | 'Host' | 'Template' | 'ISO';

export interface UnifiedBackup {
  backupType: BackupType;
  vmid: number;
  name: string;
  type: GuestType;
  node: string;
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
}
