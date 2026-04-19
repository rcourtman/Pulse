export type SystemManageAction = { kind: 'connection'; connectionId: string };

export interface InfrastructureSystemRow {
  id: string;
  name: string;
  subtitle?: string;
  host?: string;
  coverageLabels: string[];
  collectionLabel: string;
  statusLabel: string;
  statusClassName: string;
  lastActivityText: string;
  manageLabel: string;
  manage: SystemManageAction;
}
