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
  samlIdpEntityId?: string;
  samlSpEntityId?: string;
  samlMetadataUrl?: string;
  samlAcsUrl?: string;
  allowedGroups?: string[];
  allowedDomains?: string[];
  allowedEmails?: string[];
}

export interface SSOProvidersResponse {
  providers: SSOProvider[];
  defaultProviderId?: string;
  allowMultipleProviders: boolean;
}

export interface SSOProviderTestResult {
  success: boolean;
  message: string;
  error?: string;
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
  samlGroupsAttr: string;
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
  samlGroupsAttr: '',
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

export const mappingsToString = (mappings?: Record<string, string>) =>
  mappings
    ? Object.entries(mappings)
        .map(([key, value]) => `${key}=${value}`)
        .join(', ')
    : '';

export const stringToMappings = (input: string) => {
  const result: Record<string, string> = {};
  splitList(input).forEach((pair) => {
    const [key, value] = pair.split('=').map((segment) => segment.trim());
    if (key && value) {
      result[key] = value;
    }
  });
  return result;
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
  samlEmailAttr: full.saml?.emailAttr || 'email',
  samlGroupsAttr: full.saml?.groupsAttr || full.groupsClaim || '',
  allowedGroups: listToString(full.allowedGroups),
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
    allowedGroups: splitList(form.allowedGroups),
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
    payload.groupsClaim = form.samlGroupsAttr.trim() || undefined;
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
    groupsAttr: form.samlGroupsAttr.trim() || undefined,
  };
  payload.groupsClaim = form.samlGroupsAttr.trim() || undefined;
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

export const canTestProviderForm = (form: ProviderForm): boolean => {
  if (form.type === 'oidc') {
    return Boolean(form.oidcIssuerUrl.trim());
  }
  return Boolean(
    form.samlIdpMetadataUrl.trim() || form.samlIdpMetadataXml.trim() || form.samlIdpSsoUrl.trim(),
  );
};

export const buildMetadataPreviewPayload = (form: ProviderForm) => ({
  type: 'saml' as const,
  metadataUrl: form.samlIdpMetadataUrl.trim(),
});
