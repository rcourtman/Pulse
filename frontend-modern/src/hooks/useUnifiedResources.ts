import { createResource } from 'solid-js';
import { apiFetch } from '@/utils/apiClient';
import type { Resource } from '@/types/resource';

const UNIFIED_RESOURCES_URL = '/api/v2/resources?type=host';

async function fetchUnifiedResources(): Promise<Resource[]> {
  const response = await apiFetch(UNIFIED_RESOURCES_URL, { cache: 'no-store' });
  if (!response.ok) {
    throw new Error('Failed to fetch unified resources');
  }

  const data = await response.json();
  if (Array.isArray(data)) {
    return data as Resource[];
  }

  if (data && Array.isArray((data as { resources?: Resource[] }).resources)) {
    return (data as { resources: Resource[] }).resources;
  }

  return [];
}

export function useUnifiedResources() {
  const [resources, { refetch, mutate }] = createResource<Resource[]>(fetchUnifiedResources, {
    initialValue: [],
  });

  return {
    resources,
    refetch,
    mutate,
    loading: () => resources.loading,
    error: () => resources.error,
  };
}

export default useUnifiedResources;
