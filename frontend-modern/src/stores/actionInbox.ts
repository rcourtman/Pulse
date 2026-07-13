import { createSignal } from 'solid-js';
import { ResourceActionsAPI } from '@/api/resourceActions';
import { logger } from '@/utils/logger';

const [pendingActionCount, setPendingActionCount] = createSignal(0);

// The decision queue requires the action-approve capability, so a session
// without it gets a terminal 402/403/404 — stop polling for the rest of the
// session instead of re-asking every refresh.
let decisionQueueUnavailable = false;

export const actionInboxStore = {
  get pendingActionCount() {
    return pendingActionCount();
  },

  async loadPendingActionCount() {
    if (decisionQueueUnavailable) return;
    try {
      const response = await ResourceActionsAPI.listPendingActions();
      setPendingActionCount(response.count ?? response.actions.length);
    } catch (cause) {
      const status = (cause as { status?: number }).status;
      if (status === 402 || status === 403 || status === 404) {
        decisionQueueUnavailable = true;
        setPendingActionCount(0);
        return;
      }
      logger.debug('Failed to load pending action count', cause);
    }
  },
};
