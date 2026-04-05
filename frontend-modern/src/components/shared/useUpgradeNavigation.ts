import { useNavigate } from '@solidjs/router';
import {
  navigateToUpgradeDestination,
  type UpgradeDestination,
} from '@/utils/upgradeNavigation';

export function useUpgradeNavigation() {
  const navigate = useNavigate();

  return (destination: UpgradeDestination) => {
    navigateToUpgradeDestination(destination, navigate);
  };
}
