import RadioTower from 'lucide-solid/icons/radio-tower';
import Shield from 'lucide-solid/icons/shield';
import ShieldCheck from 'lucide-solid/icons/shield-check';
import Lock from 'lucide-solid/icons/lock';
import Key from 'lucide-solid/icons/key';
import Activity from 'lucide-solid/icons/activity';
import Network from 'lucide-solid/icons/network';
import Bot from 'lucide-solid/icons/bot';
import Users from 'lucide-solid/icons/users';
import Sliders from 'lucide-solid/icons/sliders-horizontal';
import RefreshCw from 'lucide-solid/icons/refresh-cw';
import Clock from 'lucide-solid/icons/clock';
import Sparkles from 'lucide-solid/icons/sparkles';

import Globe from 'lucide-solid/icons/globe';
import { ProxmoxIcon } from '@/components/icons/ProxmoxIcon';
import { PulseLogoIcon } from '@/components/icons/PulseLogoIcon';
import BadgeCheck from 'lucide-solid/icons/badge-check';
import type { SecurityStatusSettingsCapabilities } from '@/types/config';

import Container from 'lucide-solid/icons/container';
import Building2 from 'lucide-solid/icons/building-2';
import Share2 from 'lucide-solid/icons/share-2';
import CreditCard from 'lucide-solid/icons/credit-card';
import { isTabLocked } from './settingsFeatureGates';
import type { SettingsNavGroup, SettingsNavItem } from './settingsTypes';
import type { SettingsTab } from './settingsTypes';

export const baseTabGroups: SettingsNavGroup[] = [
  {
    id: 'resources',
    label: 'Resources',
    items: [
      { id: 'agents', label: 'Infrastructure', icon: Bot, iconProps: { strokeWidth: 2 } },
      { id: 'proxmox', label: 'API Connections', icon: ProxmoxIcon },
      { id: 'docker', label: 'Docker', icon: Container, iconProps: { strokeWidth: 2 } },
    ],
  },
  {
    id: 'organization',
    label: 'Organization',
    items: [
      {
        id: 'organization-overview',
        label: 'Overview',
        icon: Building2,
        iconProps: { strokeWidth: 2 },
        features: ['multi_tenant'],
        hideWhenUnavailable: true,
      },
      {
        id: 'organization-access',
        label: 'Access',
        icon: Users,
        iconProps: { strokeWidth: 2 },
        features: ['multi_tenant'],
        hideWhenUnavailable: true,
      },
      {
        id: 'organization-sharing',
        label: 'Sharing',
        icon: Share2,
        iconProps: { strokeWidth: 2 },
        features: ['multi_tenant'],
        hideWhenUnavailable: true,
      },
      {
        id: 'organization-billing',
        label: 'Billing',
        icon: CreditCard,
        iconProps: { strokeWidth: 2 },
        features: ['multi_tenant'],
        hideWhenUnavailable: true,
      },
      {
        id: 'organization-billing-admin',
        label: 'Billing Admin',
        icon: CreditCard,
        iconProps: { strokeWidth: 2 },
        features: ['multi_tenant'],
        hideWhenUnavailable: true,
        hostedOnly: true,
        requiredCapability: 'billingAdmin',
      },
    ],
  },
  {
    id: 'integrations',
    label: 'Integrations',
    items: [
      {
        id: 'api',
        label: 'API Access',
        icon: BadgeCheck,
        requiredCapability: 'apiAccessRead',
      },
    ],
  },
  {
    id: 'platform',
    label: 'Platform Administration',
    items: [
      // System grouped tab items
      {
        id: 'system-general',
        label: 'General',
        icon: Sliders,
        iconProps: { strokeWidth: 2 },
        saveBehavior: 'system',
      },
      {
        id: 'system-network',
        label: 'Network',
        icon: Network,
        iconProps: { strokeWidth: 2 },
        saveBehavior: 'system',
      },
      {
        id: 'system-updates',
        label: 'Updates',
        icon: RefreshCw,
        iconProps: { strokeWidth: 2 },
        saveBehavior: 'system',
      },
      {
        id: 'system-recovery',
        label: 'Recovery',
        icon: Clock,
        iconProps: { strokeWidth: 2 },
        saveBehavior: 'system',
      },
      {
        id: 'system-ai',
        label: 'AI Services',
        icon: Sparkles,
        iconProps: { strokeWidth: 2 },
      },
      {
        id: 'system-relay',
        label: 'Remote Access',
        icon: RadioTower,
        iconProps: { strokeWidth: 2 },
        features: ['relay'],
        requiredCapability: 'relayRead',
      },
      // Security grouped tab items
      {
        id: 'security-overview',
        label: 'Security Overview',
        icon: Shield,
        iconProps: { strokeWidth: 2 },
      },
      {
        id: 'security-auth',
        label: 'Authentication',
        icon: Lock,
        iconProps: { strokeWidth: 2 },
        requiredCapability: 'authenticationRead',
      },
      {
        id: 'security-sso',
        label: 'Single Sign-On',
        icon: Key,
        iconProps: { strokeWidth: 2 },
        requiredCapability: 'singleSignOnRead',
      },
      {
        id: 'security-roles',
        label: 'Roles',
        icon: ShieldCheck,
        iconProps: { strokeWidth: 2 },
        requiredCapability: 'roles',
      },
      {
        id: 'security-users',
        label: 'Users',
        icon: Users,
        iconProps: { strokeWidth: 2 },
        requiredCapability: 'users',
      },
      {
        id: 'security-audit',
        label: 'Audit Log',
        icon: Activity,
        iconProps: { strokeWidth: 2 },
        requiredCapability: 'auditLog',
      },
      {
        id: 'security-webhooks',
        label: 'Audit Webhooks',
        icon: Globe,
        iconProps: { strokeWidth: 2 },
        features: ['audit_logging'],
        requiredCapability: 'auditWebhooksRead',
      },
      {
        id: 'system-pro',
        label: 'Pulse Pro',
        icon: PulseLogoIcon,
      },
    ],
  },
];

export interface SettingsNavVisibilityContext {
  hasFeature: (feature: string) => boolean;
  licenseLoaded: () => boolean;
  hostedModeEnabled?: boolean;
  settingsCapabilities?: Partial<SecurityStatusSettingsCapabilities> | null;
  settingsCapabilitiesResolved?: boolean;
}

const navItemsByTab = new Map<SettingsTab, SettingsNavItem>(
  baseTabGroups.flatMap((group) => group.items.map((item) => [item.id, item] as const)),
);

export function getSettingsNavItem(tab: SettingsTab): SettingsNavItem | undefined {
  return navItemsByTab.get(tab);
}

export function getSettingsTabSaveBehavior(tab: SettingsTab): SettingsNavItem['saveBehavior'] {
  return navItemsByTab.get(tab)?.saveBehavior;
}

function hasRequiredFeatures(
  item: SettingsNavItem | undefined,
  hasFeature: (feature: string) => boolean,
): boolean {
  const requiredFeatures = item?.features ?? [];
  return requiredFeatures.every((feature) => hasFeature(feature));
}

export function shouldHideSettingsNavItem(
  tab: SettingsTab,
  context: SettingsNavVisibilityContext,
): boolean {
  const item = navItemsByTab.get(tab);
  if (!item) return false;

  if (item.hostedOnly && !context.hostedModeEnabled) {
    return true;
  }

  if (
    item.requiredCapability &&
    context.settingsCapabilitiesResolved &&
    context.settingsCapabilities?.[item.requiredCapability] !== true
  ) {
    return true;
  }

  if (item.hideWhenUnavailable && !hasRequiredFeatures(item, context.hasFeature)) {
    return true;
  }

  return false;
}

export function isSettingsNavItemLocked(
  tab: SettingsTab,
  context: SettingsNavVisibilityContext,
): boolean {
  const item = navItemsByTab.get(tab);
  if (!item || item.hideWhenUnavailable) {
    return false;
  }

  return isTabLocked(tab, context.hasFeature, context.licenseLoaded);
}
