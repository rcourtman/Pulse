import { createSignal } from 'solid-js';
import { logger } from '@/utils/logger';

// Whether an AI provider is enabled and configured for this session, from the
// security status session capability (`assistantEnabled`). Patrol, the Patrol
// digest and AI-backed suggestions gate on it. null until the first security
// status arrives.
const [aiEnabled, setAiEnabled] = createSignal<boolean | null>(null);
// Each set or refresh takes a ticket; a refresh applies its answer only if no
// newer one started meanwhile, so switching organizations A then B cannot
// end with A's late answer.
let latestTicket = 0;

export const aiAvailabilityStore = {
  get enabled() {
    return aiEnabled();
  },

  setEnabled(enabled: boolean) {
    latestTicket += 1;
    setAiEnabled(enabled);
  },

  // Forget the answer when the organization changes: Patrol entry points stay
  // hidden until the incoming organization's refresh lands, instead of showing
  // the outgoing one's.
  clear() {
    latestTicket += 1;
    setAiEnabled(null);
  },

  // The bootstrap only reads the capability on page load, so AI settings
  // saves call this to reveal (or hide) Patrol entry points without a reload.
  async refreshEnabledFromServer() {
    const ticket = ++latestTicket;
    try {
      const { SecurityAPI } = await import('@/api/security');
      const status = await SecurityAPI.getStatus();
      if (ticket !== latestTicket) return;
      setAiEnabled(status?.sessionCapabilities?.assistantEnabled === true);
    } catch (error) {
      logger.error('[aiAvailability] Failed to refresh AI availability', error);
    }
  },
};
