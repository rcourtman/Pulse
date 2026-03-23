import { createSignal } from 'solid-js';
import { createLocalStorageBooleanSignal, STORAGE_KEYS } from '@/utils/localStorage';

export function useWhatsNewModalState() {
  const [hasSeen, setHasSeen] = createLocalStorageBooleanSignal(
    STORAGE_KEYS.WHATS_NEW_NAV_V2_SHOWN,
    false,
  );
  const [dontShowAgain, setDontShowAgain] = createSignal(true);
  const [dismissedForSession, setDismissedForSession] = createSignal(false);

  const isOpen = () => !hasSeen() && !dismissedForSession();

  const handleClose = () => {
    if (dontShowAgain()) {
      setHasSeen(true);
      return;
    }

    setDismissedForSession(true);
  };

  return {
    dontShowAgain,
    handleClose,
    isOpen,
    setDontShowAgain,
  };
}
