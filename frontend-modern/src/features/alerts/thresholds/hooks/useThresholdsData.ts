import type { ThresholdsTableProps } from '../types';
import { useThresholdsDockerData } from './useThresholdsDockerData';
import { useThresholdsGuestData } from './useThresholdsGuestData';
import { useThresholdsHostData } from './useThresholdsHostData';
import { useThresholdsInfrastructureData } from './useThresholdsInfrastructureData';

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

  return {
    ...hostData,
    ...dockerData,
    ...guestData,
    ...infrastructureData,
  };
}
