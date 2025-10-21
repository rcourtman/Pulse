import type { JSX } from 'solid-js';
import type { Alert } from '@/types/api';
import type { ActivationState, AlertConfig } from '@/types/alerts';

interface ActivationBannerProps {
  activationState: () => ActivationState | null;
  activeAlerts: () => Alert[] | undefined;
  config: () => AlertConfig | null;
  isPastObservationWindow: () => boolean;
  isLoading: () => boolean;
  refreshActiveAlerts: () => Promise<void>;
  activate: () => Promise<boolean>;
}

export function ActivationBanner(_props: ActivationBannerProps): JSX.Element {
  // Notifications activation banner/modal intentionally disabled.
  return <></>;
}
