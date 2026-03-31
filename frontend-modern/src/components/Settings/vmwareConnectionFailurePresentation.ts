export interface VMwareConnectionFailurePresentation {
  code?: string;
  category?: string;
  guidance?: string;
  message: string;
  title: string;
  tone: 'danger' | 'warning';
}

interface VMwareConnectionFailurePresentationInput {
  code?: string | null | undefined;
  category?: string | null | undefined;
  message?: string | null | undefined;
  fallback: string;
  defaultGuidance?: string;
  defaultTitle?: string;
  defaultTone?: 'danger' | 'warning';
}

const normalizeOptionalText = (value: string | null | undefined): string | undefined => {
  const trimmed = value?.trim();
  return trimmed ? trimmed : undefined;
};

export const buildVMwareConnectionFailurePresentation = (
  input: VMwareConnectionFailurePresentationInput,
): VMwareConnectionFailurePresentation => {
  const code = normalizeOptionalText(input.code);
  const category = normalizeOptionalText(input.category);
  const message = normalizeOptionalText(input.message) ?? input.fallback;

  switch (category) {
    case 'unsupported_version':
      return {
        code,
        category,
        guidance:
          'Use a supported vCenter release within the current VI JSON phase-1 floor, then retry this connection test.',
        message,
        title: 'Unsupported vCenter version',
        tone: 'warning',
      };
    case 'tls':
      return {
        code,
        category,
        guidance:
          'Install a trusted certificate for vCenter, or enable Skip TLS verification only for controlled lab environments.',
        message,
        title: 'TLS validation failed',
        tone: 'warning',
      };
    case 'auth':
      return {
        code,
        category,
        guidance: 'Verify the username, password, and account scope in vCenter before retrying.',
        message,
        title: 'Authentication failed',
        tone: 'danger',
      };
    case 'permission':
      return {
        code,
        category,
        guidance:
          'Grant the minimum VMware read privileges required for phase-1 inventory and health reads, then retry.',
        message,
        title: 'Permissions are insufficient',
        tone: 'warning',
      };
    case 'network':
      return {
        code,
        category,
        guidance:
          'Confirm DNS, reachability, port 443, and any firewall rules from the Pulse server to vCenter.',
        message,
        title: 'Pulse could not reach vCenter',
        tone: 'danger',
      };
    default:
      break;
  }

  if (code === 'vmware_invalid_config') {
    return {
      code,
      category,
      guidance: 'Review the host, port, username, and password fields before retrying.',
      message,
      title: 'Connection configuration is invalid',
      tone: 'danger',
    };
  }

  return {
    code,
    category,
    guidance:
      input.defaultGuidance ??
      'Review the vCenter endpoint and credentials, then retry the connection test.',
    message,
    title: input.defaultTitle ?? 'VMware connection test failed',
    tone: input.defaultTone ?? 'danger',
  };
};
