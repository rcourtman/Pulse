import RadioTower from 'lucide-solid/icons/radio-tower';
import Shield from 'lucide-solid/icons/shield';
import ShieldCheck from 'lucide-solid/icons/shield-check';
import Lock from 'lucide-solid/icons/lock';
import Key from 'lucide-solid/icons/key';
import Activity from 'lucide-solid/icons/activity';
import Network from 'lucide-solid/icons/network';
import Server from 'lucide-solid/icons/server';
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
import FileText from 'lucide-solid/icons/file-text';
import Terminal from 'lucide-solid/icons/terminal';
import { PulseLogoIcon } from '@/components/icons/PulseLogoIcon';
import { t, type I18nMessageKey, type SupportedLocale } from '@/i18n';
import { SELF_HOSTED_PRO_BILLING_PRESENTATION } from './selfHostedBillingPresentation';
import type {
  SettingsNavGroup,
  SettingsNavGroupId,
  SettingsNavItem,
  SettingsTab,
} from './settingsNavigationModel';

export const SETTINGS_NAV_GROUPS: SettingsNavGroup[] = [
  {
    id: 'infrastructure',
    label: 'Infrastructure',
    items: [
      {
        id: 'infrastructure-systems',
        label: 'Infrastructure',
        icon: Server,
        iconProps: { strokeWidth: 2 },
      },
    ],
  },
  {
    id: 'monitoring',
    label: 'Monitoring',
    items: [
      {
        id: 'monitoring-availability',
        label: 'Availability checks',
        icon: Activity,
        iconProps: { strokeWidth: 2 },
      },
    ],
  },
  {
    id: 'pulse-intelligence',
    label: 'Pulse Intelligence',
    items: [
      {
        id: 'system-ai',
        label: 'Provider & Models',
        icon: Sparkles,
        iconProps: { strokeWidth: 2 },
      },
      {
        id: 'system-ai-patrol',
        label: 'Patrol',
        icon: Activity,
        iconProps: { strokeWidth: 2 },
      },
      {
        id: 'system-billing',
        label: SELF_HOSTED_PRO_BILLING_PRESENTATION.navLabel,
        icon: PulseLogoIcon,
        hideWhenCommercialHidden: true,
      },
      {
        id: 'system-ai-assistant',
        label: 'Assistant',
        icon: Terminal,
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
        hideWhenOrganizationHidden: true,
        hideWhenUnavailable: true,
      },
      {
        id: 'organization-access',
        label: 'Access',
        icon: Users,
        iconProps: { strokeWidth: 2 },
        features: ['multi_tenant'],
        hideWhenOrganizationHidden: true,
        hideWhenUnavailable: true,
      },
      {
        id: 'organization-sharing',
        label: 'Sharing',
        icon: Share2,
        iconProps: { strokeWidth: 2 },
        features: ['multi_tenant'],
        hideWhenOrganizationHidden: true,
        hideWhenUnavailable: true,
      },
      {
        id: 'organization-billing',
        label: 'Billing',
        icon: CreditCard,
        iconProps: { strokeWidth: 2 },
        features: ['multi_tenant'],
        hideWhenOrganizationHidden: true,
        hideWhenUnavailable: true,
        hideWhenCommercialHidden: true,
      },
      {
        id: 'organization-billing-admin',
        label: 'Billing Admin',
        icon: CreditCard,
        iconProps: { strokeWidth: 2 },
        features: ['multi_tenant'],
        hideWhenOrganizationHidden: true,
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
    ],
  },
  {
    id: 'support',
    label: 'Support',
    items: [
      {
        id: 'support-diagnostics',
        label: 'Diagnostics & Health',
        icon: Activity,
        iconProps: { strokeWidth: 2 },
        hideWhenDemoMode: true,
      },
      {
        id: 'support-reporting',
        label: 'Data & Reports',
        icon: FileText,
        iconProps: { strokeWidth: 2 },
        features: ['advanced_reporting'],
        hideWhenUnavailable: true,
        hideWhenDemoMode: true,
      },
      {
        id: 'support-logs',
        label: 'System Logs',
        icon: Terminal,
        iconProps: { strokeWidth: 2 },
        hideWhenDemoMode: true,
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
        id: 'security-data-handling',
        label: 'Resource Privacy',
        icon: FileText,
        iconProps: { strokeWidth: 2 },
        hideFromSidebar: true,
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
        features: ['rbac'],
        hideWhenUnavailable: true,
        requiredCapability: 'roles',
      },
      {
        id: 'security-users',
        label: 'Users',
        icon: Users,
        iconProps: { strokeWidth: 2 },
        features: ['rbac'],
        hideWhenUnavailable: true,
        requiredCapability: 'users',
      },
      {
        id: 'security-audit',
        label: 'Audit Log',
        icon: Activity,
        iconProps: { strokeWidth: 2 },
        features: ['audit_logging'],
        hideWhenUnavailable: true,
        requiredCapability: 'auditLog',
      },
      {
        id: 'security-webhooks',
        label: 'Audit Webhooks',
        icon: Globe,
        iconProps: { strokeWidth: 2 },
        features: ['audit_logging'],
        hideWhenUnavailable: true,
        requiredCapability: 'auditWebhooksRead',
      },
      {
        id: 'system-relay',
        label: 'Remote Access',
        icon: RadioTower,
        iconProps: { strokeWidth: 2 },
        features: ['relay'],
        // Deliberately visible without the relay feature: the panel renders its
        // own upgrade gate, and hiding the item made Relay undiscoverable for
        // free installs.
        requiredCapability: 'relayRead',
      },
    ],
  },
];

const SETTINGS_NAV_GROUP_LABEL_KEYS = {
  infrastructure: 'settings.nav.group.infrastructure',
  monitoring: 'settings.nav.group.monitoring',
  'pulse-intelligence': 'settings.nav.group.pulseIntelligence',
  organization: 'settings.nav.group.organization',
  system: 'settings.nav.group.system',
  support: 'settings.nav.group.support',
  security: 'settings.nav.group.security',
} as const satisfies Record<SettingsNavGroupId, I18nMessageKey>;

const SETTINGS_NAV_ITEM_LABEL_KEYS = {
  'infrastructure-systems': 'settings.nav.item.infrastructure',
  'monitoring-availability': 'settings.nav.item.availabilityChecks',
  'system-general': 'settings.nav.item.general',
  'system-network': 'settings.nav.item.network',
  'system-updates': 'settings.nav.item.updates',
  'system-recovery': 'settings.nav.item.recovery',
  'system-ai': 'settings.nav.item.providerModels',
  'system-ai-patrol': 'settings.nav.item.patrol',
  'system-ai-assistant': 'settings.nav.item.assistant',
  'system-ai-discovery': 'settings.nav.item.discovery',
  'system-relay': 'settings.nav.item.remoteAccess',
  'system-billing': 'settings.nav.item.plans',
  'support-diagnostics': 'settings.nav.item.diagnosticsHealth',
  'support-reporting': 'settings.nav.item.dataReports',
  'support-logs': 'settings.nav.item.systemLogs',
  'organization-overview': 'settings.nav.item.organizationOverview',
  'organization-access': 'settings.nav.item.organizationAccess',
  'organization-billing': 'settings.nav.item.billing',
  'organization-billing-admin': 'settings.nav.item.billingAdmin',
  'organization-sharing': 'settings.nav.item.sharing',
  api: 'settings.nav.item.apiAccess',
  'security-overview': 'settings.nav.item.securityOverview',
  'security-data-handling': 'settings.nav.item.resourcePrivacy',
  'security-auth': 'settings.nav.item.authentication',
  'security-sso': 'settings.nav.item.singleSignOn',
  'security-roles': 'settings.nav.item.roles',
  'security-users': 'settings.nav.item.users',
  'security-audit': 'settings.nav.item.auditLog',
  'security-webhooks': 'settings.nav.item.auditWebhooks',
} as const satisfies Record<SettingsTab, I18nMessageKey>;

function localizeSettingsNavItem(item: SettingsNavItem, locale?: SupportedLocale): SettingsNavItem {
  return {
    ...item,
    label: t(SETTINGS_NAV_ITEM_LABEL_KEYS[item.id], {}, locale),
  };
}

export function getSettingsNavGroups(locale?: SupportedLocale): SettingsNavGroup[] {
  return SETTINGS_NAV_GROUPS.map((group) => ({
    ...group,
    label: t(SETTINGS_NAV_GROUP_LABEL_KEYS[group.id], {}, locale),
    items: group.items.map((item) => localizeSettingsNavItem(item, locale)),
  }));
}

const settingsNavItemsByTab = new Map<SettingsTab, SettingsNavItem>(
  SETTINGS_NAV_GROUPS.flatMap((group) => group.items.map((item) => [item.id, item] as const)),
);

export function getSettingsNavItem(
  tab: SettingsTab,
  locale?: SupportedLocale,
): SettingsNavItem | undefined {
  const item = settingsNavItemsByTab.get(tab);
  return item ? localizeSettingsNavItem(item, locale) : undefined;
}
