import { splitProps, type Component } from 'solid-js';
import type { OrganizationRole, OrganizationShareStatus } from '@/api/orgs';
import {
  MetadataBadge,
  type MetadataBadgeProps,
  type MetadataBadgeTone,
} from '@/components/shared/MetadataBadge';
import { getOrganizationShareStatusLabel } from '@/utils/organizationSettingsPresentation';

type OrganizationBadgeProps = Omit<
  MetadataBadgeProps,
  'children' | 'tone' | 'shape' | 'uppercase' | 'role'
>;

export function getOrganizationRoleBadgeTone(role: string): MetadataBadgeTone {
  switch (role) {
    case 'owner':
      return 'warning';
    case 'admin':
      return 'info';
    case 'editor':
      return 'success';
    case 'viewer':
    default:
      return 'muted';
  }
}

export function getOrganizationShareStatusBadgeTone(
  status: OrganizationShareStatus,
): MetadataBadgeTone {
  return status === 'pending' ? 'warning' : 'success';
}

export const OrganizationRoleBadge: Component<
  OrganizationBadgeProps & { role: OrganizationRole | string }
> = (props) => {
  const [local, badgeProps] = splitProps(props, ['role']);

  return (
    <MetadataBadge {...badgeProps} tone={getOrganizationRoleBadgeTone(local.role)} shape="pill">
      {local.role}
    </MetadataBadge>
  );
};

export const OrganizationShareStatusBadge: Component<
  OrganizationBadgeProps & { status: OrganizationShareStatus }
> = (props) => {
  const [local, badgeProps] = splitProps(props, ['status']);

  return (
    <MetadataBadge
      {...badgeProps}
      tone={getOrganizationShareStatusBadgeTone(local.status)}
      shape="pill"
      fit
    >
      {getOrganizationShareStatusLabel(local.status)}
    </MetadataBadge>
  );
};
