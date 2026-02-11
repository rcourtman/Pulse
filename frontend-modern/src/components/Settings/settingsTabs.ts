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
import FileText from 'lucide-solid/icons/file-text';
import Globe from 'lucide-solid/icons/globe';
import { ProxmoxIcon } from '@/components/icons/ProxmoxIcon';
import { PulseLogoIcon } from '@/components/icons/PulseLogoIcon';
import BadgeCheck from 'lucide-solid/icons/badge-check';
import Terminal from 'lucide-solid/icons/terminal';
import Container from 'lucide-solid/icons/container';
import Building2 from 'lucide-solid/icons/building-2';
import Share2 from 'lucide-solid/icons/share-2';
import CreditCard from 'lucide-solid/icons/credit-card';
import type { SettingsNavGroup } from './settingsTypes';

export const baseTabGroups: SettingsNavGroup[] = [
  {
    id: 'resources',
    label: 'Resources',
    items: [
      { id: 'proxmox', label: 'Infrastructure', icon: ProxmoxIcon },
      { id: 'agents', label: 'Unified Agents', icon: Bot, iconProps: { strokeWidth: 2 } },
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
      },
      {
        id: 'organization-access',
        label: 'Access',
        icon: Users,
        iconProps: { strokeWidth: 2 },
        features: ['multi_tenant'],
      },
      {
        id: 'organization-sharing',
        label: 'Sharing',
        icon: Share2,
        iconProps: { strokeWidth: 2 },
        features: ['multi_tenant'],
      },
      {
        id: 'organization-billing',
        label: 'Billing',
        icon: CreditCard,
        iconProps: { strokeWidth: 2 },
        features: ['multi_tenant'],
      },
      {
        id: 'organization-billing-admin',
        label: 'Billing Admin',
        icon: CreditCard,
        iconProps: { strokeWidth: 2 },
        features: ['multi_tenant'],
        hostedOnly: true,
        adminOnly: true,
      },
    ],
  },
  {
    id: 'integrations',
    label: 'Integrations',
    items: [{ id: 'api', label: 'API Access', icon: BadgeCheck }],
  },
  {
    id: 'operations',
    label: 'Operations',
    items: [
      {
        id: 'diagnostics',
        label: 'Diagnostics',
        icon: Activity,
        iconProps: { strokeWidth: 2 },
      },
      {
        id: 'reporting',
        label: 'Reporting',
        icon: FileText,
        iconProps: { strokeWidth: 2 },
        features: ['advanced_reporting'],
      },
      {
        id: 'system-logs',
        label: 'System Logs',
        icon: Terminal,
        iconProps: { strokeWidth: 2 },
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
      },
      {
        id: 'system-network',
        label: 'Network',
        icon: Network,
        iconProps: { strokeWidth: 2 },
      },
      {
        id: 'system-updates',
        label: 'Updates',
        icon: RefreshCw,
        iconProps: { strokeWidth: 2 },
      },
      {
        id: 'system-backups',
        label: 'Backups',
        icon: Clock,
        iconProps: { strokeWidth: 2 },
      },
      {
        id: 'system-ai',
        label: 'AI',
        icon: Sparkles,
        iconProps: { strokeWidth: 2 },
      },
      {
        id: 'system-relay',
        label: 'Remote Access',
        icon: RadioTower,
        iconProps: { strokeWidth: 2 },
        features: ['relay'],
      },
      {
        id: 'system-pro',
        label: 'Pro',
        icon: PulseLogoIcon,
      },
    ],
  },
  {
    id: 'security',
    label: 'Security',
    items: [
      {
        id: 'security-overview',
        label: 'Overview',
        icon: Shield,
        iconProps: { strokeWidth: 2 },
      },
      {
        id: 'security-auth',
        label: 'Authentication',
        icon: Lock,
        iconProps: { strokeWidth: 2 },
      },
      {
        id: 'security-sso',
        label: 'Single Sign-On',
        icon: Key,
        iconProps: { strokeWidth: 2 },
      },
      {
        id: 'security-roles',
        label: 'Roles',
        icon: ShieldCheck,
        iconProps: { strokeWidth: 2 },
      },
      {
        id: 'security-users',
        label: 'Users',
        icon: Users,
        iconProps: { strokeWidth: 2 },
      },
      {
        id: 'security-audit',
        label: 'Audit Log',
        icon: Activity,
        iconProps: { strokeWidth: 2 },
      },
      {
        id: 'security-webhooks',
        label: 'Audit Webhooks',
        icon: Globe,
        iconProps: { strokeWidth: 2 },
        features: ['audit_logging'],
      },
    ],
  },
];
