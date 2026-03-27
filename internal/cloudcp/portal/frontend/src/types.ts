export interface PortalWorkspaceSummary {
  id: string;
  display_name: string;
  state: string;
  healthy: boolean;
  health_status: 'healthy' | 'checking' | 'unhealthy';
  last_health_check?: string;
  created_at?: string;
}

export interface PortalAccountSummary {
  id: string;
  name: string;
  kind: string;
  kind_label: string;
  role: string;
  can_manage: boolean;
  has_billing: boolean;
  workspaces: PortalWorkspaceSummary[];
}

export interface PortalBootstrapData {
  authenticated: boolean;
  email: string;
  public_site_url: string;
  support_email: string;
  commercial_api_base_url: string;
  portal_path: string;
  bootstrap_path: string;
  magic_link_request_path: string;
  signup_path: string;
  logout_path: string;
  account_api_base_path: string;
  portal_api_base_path: string;
  accounts: PortalAccountSummary[];
}

export interface PortalLoginState {
  emailValue: string;
  request: PortalMutationState;
  success: boolean;
}

export interface PortalTeamMember {
  email: string;
  role: string;
  user_id: string;
}

export type AsyncStatus = 'idle' | 'loading' | 'ready' | 'error';

export interface PortalQueryState<T> {
  status: AsyncStatus;
  data: T;
  error: string;
}

export interface PortalMutationState {
  pending: boolean;
  error: string;
}

export interface PortalAccountUIEntry {
  addWorkspaceOpen: boolean;
  createWorkspace: PortalMutationState;
  teamVisible: boolean;
  teamQuery: PortalQueryState<PortalTeamMember[]>;
}

export interface PortalAccountState {
  byAccountID: Record<string, PortalAccountUIEntry>;
}

export type PortalServiceFlowID = 'manage' | 'retrieve' | 'export' | 'delete';

export interface ServiceStatus {
  visible: boolean;
  message: string;
  error: boolean;
}

export interface VerificationFlowState {
  pendingEmail: string;
  request: PortalMutationState;
  confirm: PortalMutationState;
  step2Visible: boolean;
  status: ServiceStatus;
  result: unknown;
  emailValue: string;
  codeValue: string;
  checkboxChecked: boolean;
}

export interface RefundState {
  emailValue: string;
  tokenValue: string;
  submit: PortalMutationState;
  status: ServiceStatus;
}

export interface PortalServiceState {
  openPanelID: string;
  flows: Record<PortalServiceFlowID, VerificationFlowState>;
  refund: RefundState;
}
