import type {
  PortalMutationState,
  PortalAccountState,
  PortalAccountUIEntry,
  PortalLoginState,
  PortalServiceFlowID,
  PortalServiceState,
  PortalQueryState,
  RefundState,
  ServiceStatus,
  PortalTeamMember,
  VerificationFlowState,
} from './types';

export function emptyStatus(): ServiceStatus {
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
  };
}

export function createPortalAccountState(): PortalAccountState {
  return {
    byAccountID: {},
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
      openWorkspaceMenuID: '',
      teamVisible: false,
      teamQuery: createQueryState<PortalTeamMember[]>([]),
    };
  }
  return accountState.byAccountID[accountID];
}

export function createPortalServiceState(): PortalServiceState {
  return {
    openPanelID: '',
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

export function syncServiceStateBootstrapEmail(serviceState: PortalServiceState, email: string): void {
  if (!serviceState.flows.manage.emailValue) serviceState.flows.manage.emailValue = email || '';
  if (!serviceState.flows.retrieve.emailValue) serviceState.flows.retrieve.emailValue = email || '';
  if (!serviceState.flows.export.emailValue) serviceState.flows.export.emailValue = email || '';
  if (!serviceState.flows.delete.emailValue) serviceState.flows.delete.emailValue = email || '';
  if (!serviceState.refund.emailValue) serviceState.refund.emailValue = email || '';
}

export function setFlowStatus(serviceState: PortalServiceState, flowID: PortalServiceFlowID, message: string, isError: boolean): void {
  serviceState.flows[flowID].status = {
    visible: true,
    message,
    error: !!isError,
  };
}

export function clearFlowStatus(serviceState: PortalServiceState, flowID: PortalServiceFlowID): void {
  serviceState.flows[flowID].status = emptyStatus();
}

export function setRefundStatus(serviceState: PortalServiceState, message: string, isError: boolean): void {
  serviceState.refund.status = {
    visible: true,
    message,
    error: !!isError,
  };
}

export function toggleServicePanelState(serviceState: PortalServiceState, panelID: string): void {
  serviceState.openPanelID = serviceState.openPanelID === panelID ? '' : panelID;
}

export function resetVerificationFlowState(serviceState: PortalServiceState, flowID: PortalServiceFlowID): void {
  var previous = serviceState.flows[flowID];
  serviceState.flows[flowID] = newVerificationFlowState();
  serviceState.flows[flowID].emailValue = previous.emailValue;
}

export function updateServiceInputValue(serviceState: PortalServiceState, inputKind: string, value: string): void {
  switch (inputKind) {
    case 'manage-email':
      serviceState.flows.manage.emailValue = value;
      return;
    case 'manage-code':
      serviceState.flows.manage.codeValue = value;
      return;
    case 'retrieve-email':
      serviceState.flows.retrieve.emailValue = value;
      return;
    case 'retrieve-code':
      serviceState.flows.retrieve.codeValue = value;
      return;
    case 'refund-email':
      serviceState.refund.emailValue = value;
      return;
    case 'refund-token':
      serviceState.refund.tokenValue = value;
      return;
    case 'data-export-email':
      serviceState.flows.export.emailValue = value;
      return;
    case 'data-export-code':
      serviceState.flows.export.codeValue = value;
      return;
    case 'data-delete-email':
      serviceState.flows.delete.emailValue = value;
      return;
    case 'data-delete-code':
      serviceState.flows.delete.codeValue = value;
      return;
    default:
      return;
  }
}

export function updateDeleteConfirmation(serviceState: PortalServiceState, checked: boolean): void {
  serviceState.flows.delete.checkboxChecked = checked;
}
