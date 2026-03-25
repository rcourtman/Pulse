import { startProTrial } from '@/stores/license';
import { getProTrialStartedMessage, getTrialStartErrorMessage } from '@/utils/upgradePresentation';

export type StartProTrialActionOutcome = 'activated' | 'redirect' | 'error';

export interface RunStartProTrialActionOptions {
  branded?: boolean;
  successMessage?: string;
  showSuccess: (message: string) => void;
  showError: (message: string) => void;
  navigate?: (actionUrl: string) => void;
}

function defaultNavigate(actionUrl: string) {
  if (typeof window === 'undefined') return;
  window.location.href = actionUrl;
}

export async function runStartProTrialAction(
  options: RunStartProTrialActionOptions,
): Promise<StartProTrialActionOutcome> {
  const { branded = false, navigate = defaultNavigate, showError, showSuccess, successMessage } =
    options;

  try {
    const result = await startProTrial();
    if (result?.outcome === 'redirect') {
      navigate(result.actionUrl);
      return 'redirect';
    }

    showSuccess(successMessage ?? getProTrialStartedMessage());
    return 'activated';
  } catch (error) {
    showError(getTrialStartErrorMessage(error, { branded }));
    return 'error';
  }
}
