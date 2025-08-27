// Unified backup types for the backup view

export interface UnifiedBackup {
  // Common fields
  backupType: 'backup' | 'snapshot' | 'pbs';
  vmid: number;
  name: string;
  type: 'VM' | 'LXC' | 'CT';
  node: string;
  backupTime: number; // Unix timestamp in seconds
  backupName: string;
  description: string;
  status: string;
  size: number | null;
  storage: string | null;
  
  // PBS specific
  datastore: string | null;
  namespace: string | null;
  verified: boolean | null;
  
  // Common flags
  protected: boolean;
  encrypted?: boolean;
  
  // UI specific
  instance?: string;
  isPBS?: boolean;
}

// PBS-specific backup file info
export interface PBSBackupFile {
  filename: string;
  size: number;
  crypt?: string;
}

// Extended PBS backup with file details
export interface PBSBackupWithFiles {
  id: string;
  instance: string;
  datastore: string;
  namespace?: string;
  backupType: string;
  vmid: number;
  backupTime: string;
  size: number;
  protected: boolean;
  verified: boolean;
  comment?: string;
  files: PBSBackupFile[];
}

// PBS datastore with snapshots
export interface PBSDatastoreSnapshot {
  id: string;
  backupTime: string;
  size: number;
  owner?: string;
  verified?: boolean;
  protected?: boolean;
  files?: PBSBackupFile[];
}

export interface PBSDatastore {
  name: string;
  total: number;
  used: number;
  free: number;
  snapshots: PBSDatastoreSnapshot[];
}

export interface PBSInstanceData {
  id: string;
  name: string;
  host: string;
  backups: PBSBackupWithFiles[];
  datastores: PBSDatastore[];
}

// Filter options for the backup view
export interface BackupFilters {
  instance: string;
  type: 'all' | 'VM' | 'LXC';
  node: string;
  storage: string;
  protected: 'all' | 'protected' | 'unprotected';
  verified: 'all' | 'verified' | 'unverified';
}

// Sorting options
export interface BackupSort {
  key: keyof UnifiedBackup;
  order: 'asc' | 'desc';
}