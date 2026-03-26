export interface PortalWorkspaceSummary {
  id: string;
  display_name: string;
  state: string;
  healthy: boolean;
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
  sending: boolean;
  success: boolean;
  error: string;
}

export interface ServiceStatus {
  visible: boolean;
  message: string;
  error: boolean;
}

export interface VerificationFlowState {
  pendingEmail: string;
  requesting: boolean;
  confirming: boolean;
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
  submitting: boolean;
  status: ServiceStatus;
}
