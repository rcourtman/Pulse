export interface PortalWorkspaceSummary {
  id: string;
  display_name: string;
  state: string;
  healthy: boolean;
  health_status: 'healthy' | 'checking' | 'unhealthy';
  last_health_check?: string;
  created_at?: string;
}

export interface PortalAccessMember {
  email: string;
  role: string;
  user_id: string;
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
  members: PortalAccessMember[];
}

export interface PortalBootstrapData {
  authenticated: boolean;
  email: string;
  has_self_hosted_commercial: boolean;
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

export interface PortalUpgradePricingFeature {
  tone: string;
  html: string;
}

export interface PortalUpgradePricingButton {
  kind: string;
  href?: string;
  className: string;
  tier?: string;
  planKey?: string;
  billingCycle?: string;
  label: string;
}

export interface PortalUpgradePricingPlan {
  badge?: string;
  highlight?: boolean;
  tierKicker: string;
  title: string;
  price: string;
  period: string;
  blurb: string;
  features: PortalUpgradePricingFeature[];
  buttons?: PortalUpgradePricingButton[];
  note?: string;
}

export interface PortalUpgradePricingModel {
  title: string;
  description: string;
  explainer?: string;
  plans: PortalUpgradePricingPlan[];
}

export interface PortalUpgradePortalHandoffModel {
  portal_handoff_id: string;
  feature?: string;
  expires_at?: number;
}

export interface PortalCheckoutSessionCreateResponse {
  url?: string;
  plan_key?: string;
  tier?: string;
  billing_cycle?: string;
}

export interface PortalLoginState {
  emailValue: string;
  request: PortalMutationState;
  success: boolean;
  successMessage: string;
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

export type PortalAccessJob = '' | 'invite' | 'change_role' | 'remove';

export interface PortalAccountUIEntry {
  addWorkspaceOpen: boolean;
  createWorkspace: PortalMutationState;
  selectedWorkspaceID: string;
  manageWorkspace: PortalMutationState;
  accessVisible: boolean;
  activeAccessJob: PortalAccessJob;
  accessQuery: PortalQueryState<PortalAccessMember[]>;
}

export interface PortalAccountState {
  byAccountID: Record<string, PortalAccountUIEntry>;
}

export type PortalShellSection = 'overview' | 'workspaces' | 'access' | 'billing' | 'support';

export interface PortalShellState {
  activeSection: PortalShellSection;
}

export type PortalBillingFlowID = 'manage' | 'retrieve' | 'export' | 'delete';

export interface BillingStatus {
  visible: boolean;
  message: string;
  error: boolean;
}

export interface VerificationFlowState {
  pendingEmail: string;
  request: PortalMutationState;
  confirm: PortalMutationState;
  step2Visible: boolean;
  status: BillingStatus;
  result: unknown;
  emailValue: string;
  codeValue: string;
  checkboxChecked: boolean;
}

export interface RefundState {
  emailValue: string;
  tokenValue: string;
  submit: PortalMutationState;
  status: BillingStatus;
}

export interface PortalBillingState {
  openBillingPanelID: string;
  upgradeFeatureKey: string;
  upgradePortalHandoffID: string;
  upgradePortalHandoff: PortalQueryState<PortalUpgradePortalHandoffModel | null>;
  upgradePricing: PortalQueryState<PortalUpgradePricingModel | null>;
  upgradeCheckout: PortalMutationState;
  flows: Record<PortalBillingFlowID, VerificationFlowState>;
  refund: RefundState;
}
