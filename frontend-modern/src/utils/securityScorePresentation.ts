import type { Component } from 'solid-js';
import Shield from 'lucide-solid/icons/shield';
import ShieldAlert from 'lucide-solid/icons/shield-alert';
import ShieldCheck from 'lucide-solid/icons/shield-check';

export interface SecurityScoreTonePresentation {
  headerBg: string;
  headerBorder: string;
  iconWrap: string;
  icon: string;
  subtitle: string;
  score: string;
  badge: string;
}

export interface SecurityScorePresentation {
  label: 'Strong' | 'Moderate' | 'Weak';
  tone: SecurityScoreTonePresentation;
  icon: 'shield-check' | 'shield' | 'shield-alert';
}

export interface SecurityWarningPresentation {
  background: string;
  border: string;
  message: string;
  messageClass: string;
}

export interface SecurityFeatureStatePresentation {
  label: 'Yes' | 'No';
  className: string;
}

export interface SecurityFeatureCardPresentation {
  cardClassName: string;
  iconClassName: string;
  statusLabel: 'Enabled' | 'Disabled';
  criticalLabelClassName: string;
}

export interface SecurityPostureStatus {
  hasAuthentication: boolean;
  ssoEnabled?: boolean;
  hasProxyAuth?: boolean;
  apiTokenConfigured: boolean;
  exportProtected: boolean;
  unprotectedExportAllowed?: boolean;
  hasHTTPS?: boolean;
  hasAuditLogging: boolean;
  requiresAuth: boolean;
  publicAccess?: boolean;
  isPrivateNetwork?: boolean;
  clientIP?: string;
}

export interface SecurityPostureItem {
  key: 'password' | 'oidc' | 'proxy' | 'token' | 'export' | 'https' | 'audit';
  label: string;
  enabled: boolean;
  description: string;
  critical: boolean;
}

export function getSecurityScorePresentation(score: number): SecurityScorePresentation {
  if (score >= 80) {
    return {
      label: 'Strong',
      icon: 'shield-check',
      tone: {
        headerBg: 'bg-emerald-50 dark:bg-emerald-950',
        headerBorder: 'border-b border-emerald-200 dark:border-emerald-800',
        iconWrap: 'bg-emerald-100 dark:bg-emerald-900',
        icon: 'text-emerald-700 dark:text-emerald-300',
        subtitle: 'text-emerald-700 dark:text-emerald-300',
        score: 'text-emerald-800 dark:text-emerald-200',
        badge: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900 dark:text-emerald-300',
      },
    };
  }

  if (score >= 50) {
    return {
      label: 'Moderate',
      icon: 'shield',
      tone: {
        headerBg: 'bg-amber-50 dark:bg-amber-950',
        headerBorder: 'border-b border-amber-200 dark:border-amber-800',
        iconWrap: 'bg-amber-100 dark:bg-amber-900',
        icon: 'text-amber-700 dark:text-amber-300',
        subtitle: 'text-amber-700 dark:text-amber-300',
        score: 'text-amber-800 dark:text-amber-200',
        badge: 'bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300',
      },
    };
  }

  return {
    label: 'Weak',
    icon: 'shield-alert',
    tone: {
      headerBg: 'bg-rose-50 dark:bg-rose-950',
      headerBorder: 'border-b border-rose-200 dark:border-rose-800',
      iconWrap: 'bg-rose-100 dark:bg-rose-900',
      icon: 'text-rose-700 dark:text-rose-300',
      subtitle: 'text-rose-700 dark:text-rose-300',
      score: 'text-rose-800 dark:text-rose-200',
      badge: 'bg-rose-100 text-rose-700 dark:bg-rose-900 dark:text-rose-300',
    },
  };
}

export function getSecurityWarningPresentation(options: {
  score: number;
  publicAccess: boolean;
  hasAuthentication: boolean;
}): SecurityWarningPresentation {
  if (options.publicAccess && !options.hasAuthentication) {
    return {
      background: 'bg-red-50 dark:bg-red-900',
      border: 'border-red-200 dark:border-red-800',
      message:
        'WARNING: PUBLIC NETWORK ACCESS DETECTED - Your Proxmox credentials are exposed to the internet!',
      messageClass: 'font-semibold text-red-700 dark:text-red-300',
    };
  }

  const posture = getSecurityScorePresentation(options.score);
  return {
    background:
      posture.label === 'Moderate'
        ? 'bg-yellow-50 dark:bg-yellow-900'
        : 'bg-red-50 dark:bg-red-900',
    border:
      posture.label === 'Moderate'
        ? 'border-yellow-200 dark:border-yellow-800'
        : 'border-red-200 dark:border-red-800',
    message:
      'Your Pulse instance is accessible without authentication. Proxmox credentials could be exposed.',
    messageClass: 'text-base-content',
  };
}

export function getSecurityScoreTextClass(score: number): string {
  return getSecurityScorePresentation(score).tone.icon;
}

export function getSecurityFeatureStatePresentation(
  enabled: boolean,
): SecurityFeatureStatePresentation {
  return {
    label: enabled ? 'Yes' : 'No',
    className: enabled ? 'text-green-600' : 'text-red-600',
  };
}

export function getSecurityScoreIconComponent(
  score: number,
): Component<{ class?: string }> {
  switch (getSecurityScorePresentation(score).icon) {
    case 'shield-check':
      return ShieldCheck;
    case 'shield':
      return Shield;
    default:
      return ShieldAlert;
  }
}

export function getSecurityFeatureCardPresentation(options: {
  enabled: boolean;
  critical: boolean;
}): SecurityFeatureCardPresentation {
  if (options.enabled) {
    return {
      cardClassName:
        'border-emerald-200 dark:border-emerald-800 bg-emerald-50 dark:bg-emerald-950',
      iconClassName: 'text-emerald-500 dark:text-emerald-400',
      statusLabel: 'Enabled',
      criticalLabelClassName: 'text-emerald-600 dark:text-emerald-400',
    };
  }

  if (options.critical) {
    return {
      cardClassName:
        'border-rose-200 dark:border-rose-800 bg-rose-50 dark:bg-rose-950',
      iconClassName: 'text-rose-500 dark:text-rose-400',
      statusLabel: 'Disabled',
      criticalLabelClassName: 'text-rose-600 dark:text-rose-400',
    };
  }

  return {
    cardClassName: 'border-border bg-surface-alt',
    iconClassName: 'text-muted',
    statusLabel: 'Disabled',
    criticalLabelClassName: 'text-muted',
  };
}

export function getSecurityPostureItems(status: SecurityPostureStatus): SecurityPostureItem[] {
  return [
    {
      key: 'password',
      label: 'Password login',
      enabled: status.hasAuthentication,
      description: status.hasAuthentication ? 'Active' : 'Not configured',
      critical: true,
    },
    {
      key: 'oidc',
      label: 'Single sign-on',
      enabled: Boolean(status.ssoEnabled),
      description: status.ssoEnabled ? 'Provider configured' : 'Not configured',
      critical: false,
    },
    {
      key: 'proxy',
      label: 'Proxy auth',
      enabled: Boolean(status.hasProxyAuth),
      description: status.hasProxyAuth ? 'Active' : 'Not configured',
      critical: false,
    },
    {
      key: 'token',
      label: 'API token',
      enabled: status.apiTokenConfigured,
      description: status.apiTokenConfigured ? 'Active' : 'Not configured',
      critical: false,
    },
    {
      key: 'export',
      label: 'Export protection',
      enabled: status.exportProtected && !status.unprotectedExportAllowed,
      description: status.unprotectedExportAllowed
        ? 'Unprotected'
        : 'Token + passphrase required',
      critical: true,
    },
    {
      key: 'https',
      label: 'HTTPS',
      enabled: Boolean(status.hasHTTPS),
      description: status.hasHTTPS ? 'Encrypted' : 'HTTP only',
      critical: true,
    },
    {
      key: 'audit',
      label: 'Audit log',
      enabled: status.hasAuditLogging,
      description: status.hasAuditLogging ? 'Active' : 'Not enabled',
      critical: false,
    },
  ];
}

export function getSecurityNetworkAccessSubtitle(status: SecurityPostureStatus): string {
  return status.publicAccess && !status.isPrivateNetwork
    ? 'Public network access detected'
    : 'Private network access';
}

export function getSecurityScoreSymbol(score: number): '✓' | '!' | '!!' {
  switch (getSecurityScorePresentation(score).icon) {
    case 'shield-check':
      return '✓';
    case 'shield':
      return '!';
    default:
      return '!!';
  }
}
