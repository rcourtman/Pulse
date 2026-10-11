import type { Component } from 'solid-js';

import { ResourceOperatorStateSection } from '@/components/Infrastructure/ResourceOperatorStateSection';
import { WebInterfaceUrlField } from '@/components/shared/WebInterfaceUrlField';
import type { WorkloadGuest } from '@/types/workloads';
import { isGuestDrawerVM, type GuestDrawerProps } from './guestDrawerModel';

interface GuestDrawerManageProps {
  guest: WorkloadGuest;
  resourceId: string;
  metadataId: string;
  targetLabel: string;
  customUrl?: string;
  onCustomUrlChange?: GuestDrawerProps['onCustomUrlChange'];
}

export const GuestDrawerManage: Component<GuestDrawerManageProps> = (props) => (
  <div class="space-y-3" data-testid="guest-manage-tab">
    <ResourceOperatorStateSection
      resourceId={props.resourceId}
      resourceType={isGuestDrawerVM(props.guest) ? 'vm' : 'system-container'}
      platformType={props.guest.platformType || 'proxmox'}
    />
    <WebInterfaceUrlField
      metadataKind="guest"
      metadataId={props.metadataId}
      targetLabel={props.targetLabel}
      customUrl={props.customUrl}
      onCustomUrlChange={(url) => props.onCustomUrlChange?.(props.metadataId, url)}
    />
  </div>
);
