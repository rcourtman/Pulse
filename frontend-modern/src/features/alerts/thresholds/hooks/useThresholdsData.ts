import type { ThresholdsTableProps } from '../types';
import { useThresholdsDockerData } from './useThresholdsDockerData';
import { useThresholdsGuestData } from './useThresholdsGuestData';
import { useThresholdsHostData } from './useThresholdsHostData';
import { useThresholdsInfrastructureData } from './useThresholdsInfrastructureData';
import { useThresholdsPlatformData } from './useThresholdsPlatformData';

export function useThresholdsData(
  props: ThresholdsTableProps,
  editingId: () => string | null,
  searchTerm: () => string,
) {
  const inputs = { props, editingId, searchTerm };
  const hostData = useThresholdsHostData(inputs);
  const dockerData = useThresholdsDockerData(inputs);
  const guestData = useThresholdsGuestData(inputs);
  const infrastructureData = useThresholdsInfrastructureData(inputs);
  const platformData = useThresholdsPlatformData(inputs);

  return {
    ...hostData,
    ...dockerData,
    ...guestData,
    ...infrastructureData,
    ...platformData,
  };
}
