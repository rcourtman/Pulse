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
        id: 'team',
        label: 'Overview',
        icon: Building2,
        iconProps: { strokeWidth: 2 },
        features: ['multi_tenant'],
      },
      {
        id: 'team',
        label: 'Access',
        icon: Users,
        iconProps: { strokeWidth: 2 },
        features: ['multi_tenant'],
      },
      {
        id: 'team',
        label: 'Sharing',
        icon: Share2,
        iconProps: { strokeWidth: 2 },
        features: ['multi_tenant'],
      },
      {
        id: 'team',
        label: 'Billing',
        icon: CreditCard,
        iconProps: { strokeWidth: 2 },
        features: ['multi_tenant'],
      },
      {
        id: 'team',
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
    items: [{ id: 'integrations', label: 'API Access', icon: BadgeCheck }],
  },
  {
    id: 'platform',
    label: 'Platform Administration',
    items: [
      // System grouped tab items
      {
        id: 'workspace',
        label: 'General',
        icon: Sliders,
        iconProps: { strokeWidth: 2 },
      },
      {
        id: 'integrations',
        label: 'Network',
        icon: Network,
        iconProps: { strokeWidth: 2 },
      },
      {
        id: 'maintenance',
        label: 'Updates',
        icon: RefreshCw,
        iconProps: { strokeWidth: 2 },
      },
      {
        id: 'maintenance',
        label: 'Recovery',
        icon: Clock,
        iconProps: { strokeWidth: 2 },
      },
      {
        id: 'workspace',
        label: 'AI Services',
        icon: Sparkles,
        iconProps: { strokeWidth: 2 },
      },
      {
        id: 'integrations',
        label: 'Remote Access',
        icon: RadioTower,
        iconProps: { strokeWidth: 2 },
        features: ['relay'],
      },
      // Security grouped tab items
      {
        id: 'authentication',
        label: 'Security Overview',
        icon: Shield,
        iconProps: { strokeWidth: 2 },
      },
      {
        id: 'authentication',
        label: 'Authentication',
        icon: Lock,
        iconProps: { strokeWidth: 2 },
      },
      {
        id: 'authentication',
        label: 'Single Sign-On',
        icon: Key,
        iconProps: { strokeWidth: 2 },
      },
      {
        id: 'team',
        label: 'Roles',
        icon: ShieldCheck,
        iconProps: { strokeWidth: 2 },
      },
      {
        id: 'team',
        label: 'Users',
        icon: Users,
        iconProps: { strokeWidth: 2 },
      },
      {
        id: 'audit',
        label: 'Audit Log',
        icon: Activity,
        iconProps: { strokeWidth: 2 },
      },
      {
        id: 'audit',
        label: 'Audit Webhooks',
        icon: Globe,
        iconProps: { strokeWidth: 2 },
        features: ['audit_logging'],
      },
      {
        id: 'workspace',
        label: 'Pulse Pro',
        icon: PulseLogoIcon,
      },
    ],
  },
];
