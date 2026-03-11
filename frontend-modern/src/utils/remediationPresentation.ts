export type RemediationPresentation = {
  errorClass: string;
  iconClass: string;
  message: string;
  messageClass: string;
  panelClass: string;
};

const SUCCESS_PRESENTATION: RemediationPresentation = {
  errorClass: 'text-red-600 dark:text-red-400',
  iconClass: 'text-green-600 dark:text-green-400',
  message: 'Fix executed successfully',
  messageClass: 'font-medium text-green-700 dark:text-green-300',
  panelClass: 'bg-green-50 dark:bg-green-900 border border-green-200 dark:border-green-800',
};

const FAILURE_PRESENTATION: RemediationPresentation = {
  errorClass: 'text-red-600 dark:text-red-400',
  iconClass: 'text-red-600 dark:text-red-400',
  message: 'Fix failed',
  messageClass: 'font-medium text-red-700 dark:text-red-300',
  panelClass: 'bg-red-50 dark:bg-red-900 border border-red-200 dark:border-red-800',
};

export function getRemediationPresentation(success: boolean): RemediationPresentation {
  return success ? SUCCESS_PRESENTATION : FAILURE_PRESENTATION;
}
