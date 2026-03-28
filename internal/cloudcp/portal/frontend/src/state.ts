import type {
  PortalMutationState,
  PortalAccountState,
  PortalAccountUIEntry,
  PortalLoginState,
  PortalShellState,
  PortalBillingFlowID,
  PortalBillingState,
  PortalQueryState,
  RefundState,
  BillingStatus,
  PortalAccessMember,
  VerificationFlowState,
} from './types';

export function emptyStatus(): BillingStatus {
  return {
    visible: false,
    message: '',
    error: false,
  };
}

export function newVerificationFlowState(): VerificationFlowState {
  return {
    pendingEmail: '',
    request: createMutationState(),
    confirm: createMutationState(),
    step2Visible: false,
    status: emptyStatus(),
    result: null,
    emailValue: '',
    codeValue: '',
    checkboxChecked: false,
  };
}

export function createPortalLoginState(): PortalLoginState {
  return {
    emailValue: '',
    request: createMutationState(),
    success: false,
    successMessage: '',
  };
}

export function createPortalAccountState(): PortalAccountState {
  return {
    byAccountID: {},
  };
}

export function createPortalShellState(): PortalShellState {
  return {
    activeSection: 'overview',
  };
}

export function createMutationState(): PortalMutationState {
  return {
    pending: false,
    error: '',
  };
}

export function createQueryState<T>(data: T): PortalQueryState<T> {
  return {
    status: 'idle',
    data: data,
    error: '',
  };
}

export function ensurePortalAccountUIEntry(accountState: PortalAccountState, accountID: string): PortalAccountUIEntry {
  if (!accountState.byAccountID[accountID]) {
    accountState.byAccountID[accountID] = {
      addWorkspaceOpen: false,
      createWorkspace: createMutationState(),
      selectedWorkspaceID: '',
      manageWorkspace: createMutationState(),
      accessVisible: false,
      accessQuery: createQueryState<PortalAccessMember[]>([]),
    };
  }
  return accountState.byAccountID[accountID];
}

export function createPortalBillingState(): PortalBillingState {
  return {
    openBillingPanelID: '',
    flows: {
      manage: newVerificationFlowState(),
      retrieve: newVerificationFlowState(),
      export: newVerificationFlowState(),
      delete: newVerificationFlowState(),
    },
    refund: {
      emailValue: '',
      tokenValue: '',
      submit: createMutationState(),
      status: emptyStatus(),
    },
  };
}

export function syncLoginStateBootstrapEmail(loginState: PortalLoginState, email: string): void {
  if (!loginState.emailValue) {
    loginState.emailValue = email || '';
  }
}

export function syncBillingStateBootstrapEmail(billingState: PortalBillingState, email: string): void {
  if (!billingState.flows.manage.emailValue) billingState.flows.manage.emailValue = email || '';
  if (!billingState.flows.retrieve.emailValue) billingState.flows.retrieve.emailValue = email || '';
  if (!billingState.flows.export.emailValue) billingState.flows.export.emailValue = email || '';
  if (!billingState.flows.delete.emailValue) billingState.flows.delete.emailValue = email || '';
  if (!billingState.refund.emailValue) billingState.refund.emailValue = email || '';
}

export function setFlowStatus(billingState: PortalBillingState, flowID: PortalBillingFlowID, message: string, isError: boolean): void {
  billingState.flows[flowID].status = {
    visible: true,
    message,
    error: !!isError,
  };
}

export function clearFlowStatus(billingState: PortalBillingState, flowID: PortalBillingFlowID): void {
  billingState.flows[flowID].status = emptyStatus();
}

export function setRefundStatus(billingState: PortalBillingState, message: string, isError: boolean): void {
  billingState.refund.status = {
    visible: true,
    message,
    error: !!isError,
  };
}

export function toggleBillingPanelState(billingState: PortalBillingState, panelID: string): void {
  billingState.openBillingPanelID = billingState.openBillingPanelID === panelID ? '' : panelID;
}

export function resetVerificationFlowState(billingState: PortalBillingState, flowID: PortalBillingFlowID): void {
  var previous = billingState.flows[flowID];
  billingState.flows[flowID] = newVerificationFlowState();
  billingState.flows[flowID].emailValue = previous.emailValue;
}

export function updateBillingInputValue(billingState: PortalBillingState, inputKind: string, value: string): void {
  switch (inputKind) {
    case 'manage-email':
      billingState.flows.manage.emailValue = value;
      return;
    case 'manage-code':
      billingState.flows.manage.codeValue = value;
      return;
    case 'retrieve-email':
      billingState.flows.retrieve.emailValue = value;
      return;
    case 'retrieve-code':
      billingState.flows.retrieve.codeValue = value;
      return;
    case 'refund-email':
      billingState.refund.emailValue = value;
      return;
    case 'refund-token':
      billingState.refund.tokenValue = value;
      return;
    case 'data-export-email':
      billingState.flows.export.emailValue = value;
      return;
    case 'data-export-code':
      billingState.flows.export.codeValue = value;
      return;
    case 'data-delete-email':
      billingState.flows.delete.emailValue = value;
      return;
    case 'data-delete-code':
      billingState.flows.delete.codeValue = value;
      return;
    default:
      return;
  }
}

export function updateDeleteConfirmation(billingState: PortalBillingState, checked: boolean): void {
  billingState.flows.delete.checkboxChecked = checked;
}
