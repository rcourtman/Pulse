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
import BadgeCheck from 'lucide-solid/icons/badge-check';
import Building2 from 'lucide-solid/icons/building-2';
import Share2 from 'lucide-solid/icons/share-2';
import CreditCard from 'lucide-solid/icons/credit-card';
import { PulseLogoIcon } from '@/components/icons/PulseLogoIcon';
import { SELF_HOSTED_PRO_BILLING_PRESENTATION } from './selfHostedBillingPresentation';
import type {
  SettingsNavGroup,
  SettingsNavItem,
  SettingsTab,
} from './settingsNavigationModel';

export const SETTINGS_NAV_GROUPS: SettingsNavGroup[] = [
  {
    id: 'infrastructure',
    label: 'Infrastructure',
    items: [
      {
        id: 'infrastructure-operations',
        label: 'Operations',
        icon: Bot,
        iconProps: { strokeWidth: 2 },
      },
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
        hideWhenCommercialHidden: true,
      },
      {
        id: 'organization-billing-admin',
        label: 'Billing Admin',
        icon: CreditCard,
        iconProps: { strokeWidth: 2 },
        features: ['multi_tenant'],
        hideWhenUnavailable: true,
        hostedOnly: true,
        hideWhenCommercialHidden: true,
        requiredCapability: 'billingAdmin',
      },
    ],
  },
  {
    id: 'system',
    label: 'System',
    items: [
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
        id: 'system-billing',
        label: SELF_HOSTED_PRO_BILLING_PRESENTATION.shellTitle,
        icon: PulseLogoIcon,
        hideWhenCommercialHidden: true,
      },
    ],
  },
  {
    id: 'security',
    label: 'Security',
    items: [
      {
        id: 'security-overview',
        label: 'Security Overview',
        icon: Shield,
        iconProps: { strokeWidth: 2 },
      },
      {
        id: 'api',
        label: 'API Access',
        icon: BadgeCheck,
        requiredCapability: 'apiAccessRead',
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
        id: 'system-relay',
        label: 'Remote Access',
        icon: RadioTower,
        iconProps: { strokeWidth: 2 },
        features: ['relay'],
        requiredCapability: 'relayRead',
      },
    ],
  },
];

const settingsNavItemsByTab = new Map<SettingsTab, SettingsNavItem>(
  SETTINGS_NAV_GROUPS.flatMap((group) => group.items.map((item) => [item.id, item] as const)),
);

export function getSettingsNavItem(tab: SettingsTab): SettingsNavItem | undefined {
  return settingsNavItemsByTab.get(tab);
}
