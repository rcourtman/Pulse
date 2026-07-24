export const RESOURCE_METADATA_CHANGED_EVENT = 'pulse:resource-metadata-changed';

export type ResourceMetadataChangedDetail = {
  metadataKind: 'agent' | 'guest' | 'docker' | 'docker-host';
  metadataId: string;
  customUrl?: string;
};

export const dispatchResourceMetadataChanged = (detail: ResourceMetadataChangedDetail): void => {
  if (typeof window === 'undefined' || !detail.metadataId) return;

  try {
    window.dispatchEvent(
      new CustomEvent<ResourceMetadataChangedDetail>(RESOURCE_METADATA_CHANGED_EVENT, {
        detail,
      }),
    );
  } catch {
    // Metadata writes remain authoritative; event dispatch is only same-tab UI synchronization.
  }
};
