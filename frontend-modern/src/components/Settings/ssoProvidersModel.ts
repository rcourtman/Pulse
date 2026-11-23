export interface SSOProvider {
  id: string;
  name: string;
  type: 'oidc' | 'saml';
  enabled: boolean;
  displayName?: string;
  iconUrl?: string;
  priority: number;
  oidcIssuerUrl?: string;
  oidcClientId?: string;
  oidcClientSecretSet?: boolean;
  oidcCallbackUrl?: string;
  samlIdpEntityId?: string;
  samlSpEntityId?: string;
  samlMetadataUrl?: string;
  samlAcsUrl?: string;
  allowedGroups?: string[];
  allowedDomains?: string[];
  allowedEmails?: string[];
  groupsClaim?: string;
  groupRoleMappings?: Record<string, string>;
}

export interface SSOProvidersResponse {
  providers: SSOProvider[];
  defaultProviderId?: string;
  allowMultipleProviders: boolean;
}

export interface SSOProviderTestResult {
  success: boolean;
  message: string;
  failureLabel?: string;
  details?: {
    type: string;
    entityId?: string;
    ssoUrl?: string;
    sloUrl?: string;
    tokenEndpoint?: string;
    userinfoEndpoint?: string;
    certificates?: Array<{
      subject: string;
      issuer: string;
      notBefore: string;
      notAfter: string;
      isExpired: boolean;
    }>;
  };
}

export interface MetadataPreview {
  xml: string;
  parsed: {
    entityId: string;
    ssoUrl?: string;
    sloUrl?: string;
    certificates?: Array<{
      subject: string;
      notAfter: string;
      isExpired?: boolean;
    }>;
    nameIdFormats?: string[];
  };
}

export interface ProviderForm {
  id: string;
  name: string;
  type: 'oidc' | 'saml';
  enabled: boolean;
  displayName: string;
  priority: number;
  oidcIssuerUrl: string;
  oidcClientId: string;
  oidcClientSecret: string;
  oidcRedirectUrl: string;
  oidcLogoutUrl: string;
  oidcScopes: string;
  samlIdpMetadataUrl: string;
  samlIdpMetadataXml: string;
  samlIdpSsoUrl: string;
  samlIdpEntityId: string;
  samlIdpCertificate: string;
  samlSpEntityId: string;
  samlSignRequests: boolean;
  samlAllowIdpInitiated: boolean;
  samlUsernameAttr: string;
  samlEmailAttr: string;
  groupsClaim: string;
  allowedGroups: string;
  allowedDomains: string;
  allowedEmails: string;
  groupRoleMappings: string;
}

interface SSOProviderDetailsResponse {
  id: string;
  name: string;
  type: 'oidc' | 'saml';
  enabled: boolean;
  displayName?: string;
  priority?: number;
  oidc?: {
    issuerUrl?: string;
    clientId?: string;
    redirectUrl?: string;
    logoutUrl?: string;
    scopes?: string[];
  };
  saml?: {
    idpMetadataUrl?: string;
    idpMetadataXml?: string;
    idpSsoUrl?: string;
    idpEntityId?: string;
    idpCertificate?: string;
    spEntityId?: string;
    signRequests?: boolean;
    allowIdpInitiated?: boolean;
    usernameAttr?: string;
    emailAttr?: string;
    groupsAttr?: string;
  };
  groupsClaim?: string;
  allowedGroups?: string[];
  allowedDomains?: string[];
  allowedEmails?: string[];
  groupRoleMappings?: Record<string, string>;
}

export const createEmptyProviderForm = (): ProviderForm => ({
  id: '',
  name: '',
  type: 'oidc',
  enabled: true,
  displayName: '',
  priority: 0,
  oidcIssuerUrl: '',
  oidcClientId: '',
  oidcClientSecret: '',
  oidcRedirectUrl: '',
  oidcLogoutUrl: '',
  oidcScopes: 'openid profile email',
  samlIdpMetadataUrl: '',
  samlIdpMetadataXml: '',
  samlIdpSsoUrl: '',
  samlIdpEntityId: '',
  samlIdpCertificate: '',
  samlSpEntityId: '',
  samlSignRequests: false,
  samlAllowIdpInitiated: false,
  samlUsernameAttr: '',
  samlEmailAttr: 'email',
  groupsClaim: '',
  allowedGroups: '',
  allowedDomains: '',
  allowedEmails: '',
  groupRoleMappings: '',
});

export const listToString = (values?: string[]) =>
  values && values.length > 0 ? values.join(', ') : '';

export const splitList = (input: string) =>
  input
    .split(/[,\s]+/)
    .map((value) => value.trim())
    .filter(Boolean);

type GroupRuleField = 'allowedGroups' | 'groupRoleMappings';

export class ProviderGroupRuleError extends Error {
  readonly field: GroupRuleField;

  constructor(field: GroupRuleField, duplicate = false) {
    super(
      field === 'allowedGroups'
        ? 'Allowed Groups: separate names with commas or new lines. Use matching double quotes around a name containing a comma or =.'
        : duplicate
          ? 'Group Role Mappings: enter each group only once.'
          : 'Group Role Mappings: enter complete group=roleId pairs. Use matching double quotes around a group name containing a comma or =.',
    );
    this.field = field;
  }
}

const formatGroupToken = (value: string) => {
  const quoted = JSON.stringify(value);
  // Quote whenever displaying the bare token could change its meaning on save.
  return !value || value.trim() !== value || /[,=]/.test(value) || quoted.slice(1, -1) !== value
    ? quoted
    : value;
};

const parseGroupEntries = (input: string, field: GroupRuleField): string[][] => {
  const invalid = () => new ProviderGroupRuleError(field);
  let offset = 0;
  const read = (pattern: RegExp) => {
    pattern.lastIndex = offset;
    const match = pattern.exec(input);
    if (match) offset = pattern.lastIndex;
    return match?.[0];
  };
  const readToken = (): string => {
    read(/[^\S\r\n]*/y);
    if (input[offset] === '"') {
      const quoted = read(/"(?:[^"\\\r\n]|\\.)*"/y);
      if (!quoted) throw invalid();
      try {
        // This token begins and ends with a double quote, so valid JSON is a string.
        return JSON.parse(quoted) as string;
      } catch {
        throw invalid();
      }
    }
    const plain = read(/[^",=\r\n]+/y)?.trim();
    if (!plain) throw invalid();
    return plain;
  };

  const entries: string[][] = [];
  read(/[,\s]*/y);
  while (offset < input.length) {
    const entry = [readToken()];
    if (field === 'groupRoleMappings') {
      read(/[^\S\r\n]*/y);
      if (!read(/=/y)) throw invalid();
      entry.push(readToken());
    }
    read(/[^\S\r\n]*/y);
    if (offset < input.length && !read(/[,\r\n][,\s]*/y)) throw invalid();
    entries.push(entry);
  }
  // A blank field deliberately clears it. Separators alone are not a group list.
  if (!entries.length && input.trim()) throw invalid();
  return entries;
};

const mappingsToString = (mappings?: Record<string, string>) =>
  Object.entries(mappings ?? {})
    .map(([group, role]) => formatGroupToken(group) + '=' + formatGroupToken(role))
    .join(', ');

const stringToMappings = (input: string) => {
  const pairs = parseGroupEntries(input, 'groupRoleMappings');
  if (new Set(pairs.map(([group]) => group)).size !== pairs.length)
    throw new ProviderGroupRuleError('groupRoleMappings', true);
  return Object.fromEntries(pairs);
};

export const mapProviderDetailsToForm = (full: SSOProviderDetailsResponse): ProviderForm => ({
  id: full.id,
  name: full.name,
  type: full.type,
  enabled: full.enabled,
  displayName: full.displayName || '',
  priority: full.priority || 0,
  oidcIssuerUrl: full.oidc?.issuerUrl || '',
  oidcClientId: full.oidc?.clientId || '',
  oidcClientSecret: '',
  oidcRedirectUrl: full.oidc?.redirectUrl || '',
  oidcLogoutUrl: full.oidc?.logoutUrl || '',
  oidcScopes: full.oidc?.scopes?.join(' ') || 'openid profile email',
  samlIdpMetadataUrl: full.saml?.idpMetadataUrl || '',
  samlIdpMetadataXml: full.saml?.idpMetadataXml || '',
  samlIdpSsoUrl: full.saml?.idpSsoUrl || '',
  samlIdpEntityId: full.saml?.idpEntityId || '',
  samlIdpCertificate: full.saml?.idpCertificate || '',
  samlSpEntityId: full.saml?.spEntityId || '',
  samlSignRequests: full.saml?.signRequests || false,
  samlAllowIdpInitiated: full.saml?.allowIdpInitiated || false,
  samlUsernameAttr: full.saml?.usernameAttr || '',
  // Only new drafts default to 'email'. Saved details omit an unset attribute.
  samlEmailAttr: full.saml?.emailAttr ?? '',
  groupsClaim: full.saml?.groupsAttr || full.groupsClaim || '',
  allowedGroups: (full.allowedGroups ?? []).map(formatGroupToken).join(', '),
  allowedDomains: listToString(full.allowedDomains),
  allowedEmails: listToString(full.allowedEmails),
  groupRoleMappings: mappingsToString(full.groupRoleMappings),
});

export const buildProviderPayload = (form: ProviderForm): Record<string, unknown> => {
  const payload: Record<string, unknown> = {
    id: form.id || undefined,
    name: form.name.trim(),
    type: form.type,
    enabled: form.enabled,
    displayName: form.displayName.trim() || undefined,
    priority: form.priority,
    allowedGroups: parseGroupEntries(form.allowedGroups, 'allowedGroups').map(([group]) => group),
    allowedDomains: splitList(form.allowedDomains),
    allowedEmails: splitList(form.allowedEmails),
    groupRoleMappings: stringToMappings(form.groupRoleMappings),
  };

  if (form.type === 'oidc') {
    payload.oidc = {
      issuerUrl: form.oidcIssuerUrl.trim(),
      clientId: form.oidcClientId.trim(),
      clientSecret: form.oidcClientSecret.trim() || undefined,
      redirectUrl: form.oidcRedirectUrl.trim() || undefined,
      logoutUrl: form.oidcLogoutUrl.trim() || undefined,
      scopes: splitList(form.oidcScopes),
    };
    payload.groupsClaim = form.groupsClaim.trim() || undefined;
    return payload;
  }

  payload.saml = {
    idpMetadataUrl: form.samlIdpMetadataUrl.trim() || undefined,
    idpMetadataXml: form.samlIdpMetadataXml.trim() || undefined,
    idpSsoUrl: form.samlIdpSsoUrl.trim() || undefined,
    idpEntityId: form.samlIdpEntityId.trim() || undefined,
    idpCertificate: form.samlIdpCertificate.trim() || undefined,
    spEntityId: form.samlSpEntityId.trim() || undefined,
    signRequests: form.samlSignRequests,
    allowIdpInitiated: form.samlAllowIdpInitiated,
    usernameAttr: form.samlUsernameAttr.trim() || undefined,
    emailAttr: form.samlEmailAttr.trim() || undefined,
    groupsAttr: form.groupsClaim.trim() || undefined,
  };
  payload.groupsClaim = form.groupsClaim.trim() || undefined;
  return payload;
};

export const buildProviderTestPayload = (form: ProviderForm): Record<string, unknown> => {
  const payload: Record<string, unknown> = {
    type: form.type,
  };

  if (form.type === 'oidc') {
    payload.oidc = {
      issuerUrl: form.oidcIssuerUrl.trim(),
      clientId: form.oidcClientId.trim() || undefined,
    };
    return payload;
  }

  payload.saml = {
    idpMetadataUrl: form.samlIdpMetadataUrl.trim() || undefined,
    idpMetadataXml: form.samlIdpMetadataXml.trim() || undefined,
    idpSsoUrl: form.samlIdpSsoUrl.trim() || undefined,
    idpCertificate: form.samlIdpCertificate.trim() || undefined,
  };
  return payload;
};

export type SSOProviderTestSource =
  'oidc-issuer' | 'saml-metadata-url' | 'saml-metadata-xml' | 'saml-manual-url';

export const getProviderTestSource = (form: ProviderForm): SSOProviderTestSource | null => {
  if (form.type === 'oidc') {
    return form.oidcIssuerUrl.trim() ? 'oidc-issuer' : null;
  }
  // Match the server's order: a metadata source takes precedence over manual fields.
  if (form.samlIdpMetadataUrl.trim()) return 'saml-metadata-url';
  if (form.samlIdpMetadataXml.trim()) return 'saml-metadata-xml';
  return form.samlIdpSsoUrl.trim() ? 'saml-manual-url' : null;
};

export const canTestProviderForm = (form: ProviderForm): boolean =>
  getProviderTestSource(form) !== null;

export const buildMetadataPreviewPayload = (form: ProviderForm) => ({
  type: 'saml' as const,
  metadataUrl: form.samlIdpMetadataUrl.trim(),
});
