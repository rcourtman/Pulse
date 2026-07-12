/**
 * Branch-coverage tests for the still-uncovered named helpers in
 * ssoProvidersModel:
 *   buildProviderTestPayload, buildMetadataPreviewPayload,
 *   mapProviderDetailsToForm, buildProviderPayload, canTestProviderForm.
 *
 * Every `||`, optional-chain, and if/else arm is driven with concrete inputs
 * and asserted against the exact emitted shape (no truthiness-only checks).
 */
import { describe, expect, it } from 'vitest';
import {
  buildMetadataPreviewPayload,
  buildProviderPayload,
  buildProviderTestPayload,
  canTestProviderForm,
  createEmptyProviderForm,
  mapProviderDetailsToForm,
  type ProviderForm,
} from '../ssoProvidersModel';

// ---- Fixtures ---------------------------------------------------------------

const oidcForm = (overrides: Partial<ProviderForm> = {}): ProviderForm => ({
  ...createEmptyProviderForm(),
  type: 'oidc' as const,
  ...overrides,
});

const samlForm = (overrides: Partial<ProviderForm> = {}): ProviderForm => ({
  ...createEmptyProviderForm(),
  type: 'saml' as const,
  ...overrides,
});

// ---- buildProviderTestPayload ----------------------------------------------

describe('buildProviderTestPayload', () => {
  it('emits the oidc arm with a populated issuerUrl and clientId', () => {
    const payload = buildProviderTestPayload(
      oidcForm({
        oidcIssuerUrl: '  https://idp.example.com  ',
        oidcClientId: '  pulse  ',
      }),
    );
    expect(payload).toStrictEqual({
      type: 'oidc',
      oidc: {
        issuerUrl: 'https://idp.example.com',
        clientId: 'pulse',
      },
    });
  });

  it('keeps issuerUrl as an empty string and collapses a blank oidc clientId to undefined', () => {
    // issuerUrl is emitted unconditionally (only trimmed); a blank clientId
    // falls through the `|| undefined` arm.
    const payload = buildProviderTestPayload(
      oidcForm({ oidcIssuerUrl: '   ', oidcClientId: '   ' }),
    );
    expect(payload).toStrictEqual({
      type: 'oidc',
      oidc: { issuerUrl: '', clientId: undefined },
    });
  });

  it('emits the saml arm with every populated field', () => {
    const payload = buildProviderTestPayload(
      samlForm({
        samlIdpMetadataUrl: 'https://idp.example.com/metadata',
        samlIdpMetadataXml: '<EntityDescriptor/>',
        samlIdpSsoUrl: 'https://idp.example.com/sso',
        samlIdpCertificate: 'cert-data',
      }),
    );
    expect(payload).toStrictEqual({
      type: 'saml',
      saml: {
        idpMetadataUrl: 'https://idp.example.com/metadata',
        idpMetadataXml: '<EntityDescriptor/>',
        idpSsoUrl: 'https://idp.example.com/sso',
        idpCertificate: 'cert-data',
      },
    });
  });

  it('collapses every blank saml field to undefined in the saml arm', () => {
    const payload = buildProviderTestPayload(
      samlForm({
        samlIdpMetadataUrl: '   ',
        samlIdpMetadataXml: '   ',
        samlIdpSsoUrl: '   ',
        samlIdpCertificate: '   ',
      }),
    );
    expect(payload).toStrictEqual({
      type: 'saml',
      saml: {
        idpMetadataUrl: undefined,
        idpMetadataXml: undefined,
        idpSsoUrl: undefined,
        idpCertificate: undefined,
      },
    });
  });
});

// ---- buildMetadataPreviewPayload --------------------------------------------

describe('buildMetadataPreviewPayload', () => {
  it('always reports type "saml" and trims the metadata url', () => {
    const payload = buildMetadataPreviewPayload(
      samlForm({ samlIdpMetadataUrl: '  https://idp.example.com/metadata  ' }),
    );
    expect(payload).toStrictEqual({
      type: 'saml',
      metadataUrl: 'https://idp.example.com/metadata',
    });
  });

  it('preserves an empty metadata url as an empty string (not undefined)', () => {
    expect(buildMetadataPreviewPayload(samlForm({ samlIdpMetadataUrl: '   ' }))).toStrictEqual({
      type: 'saml',
      metadataUrl: '',
    });
  });
});

// ---- mapProviderDetailsToForm -----------------------------------------------

describe('mapProviderDetailsToForm', () => {
  it('maps a fully-populated SAML provider details response into the form', () => {
    const form = mapProviderDetailsToForm({
      id: 'saml-1',
      name: 'Corp SAML',
      type: 'saml',
      enabled: true,
      displayName: 'Corp',
      priority: 7,
      saml: {
        idpMetadataUrl: 'https://idp.example.com/metadata',
        idpMetadataXml: '<EntityDescriptor/>',
        idpSsoUrl: 'https://idp.example.com/sso',
        idpEntityId: 'https://idp.example.com',
        idpCertificate: 'cert-data',
        spEntityId: 'https://sp.example.com',
        signRequests: true,
        allowIdpInitiated: true,
        usernameAttr: 'user',
        emailAttr: 'mail',
        groupsAttr: 'memberOf',
      },
      groupsClaim: 'ignored-when-groupsAttr-set',
      allowedGroups: ['admins', 'operators'],
      allowedDomains: ['example.com'],
      allowedEmails: ['u@example.com'],
      groupRoleMappings: { admins: 'admin' },
    });

    expect(form).toStrictEqual({
      id: 'saml-1',
      name: 'Corp SAML',
      type: 'saml',
      enabled: true,
      displayName: 'Corp',
      priority: 7,
      oidcIssuerUrl: '',
      oidcClientId: '',
      oidcClientSecret: '',
      oidcRedirectUrl: '',
      oidcLogoutUrl: '',
      oidcScopes: 'openid profile email',
      samlIdpMetadataUrl: 'https://idp.example.com/metadata',
      samlIdpMetadataXml: '<EntityDescriptor/>',
      samlIdpSsoUrl: 'https://idp.example.com/sso',
      samlIdpEntityId: 'https://idp.example.com',
      samlIdpCertificate: 'cert-data',
      samlSpEntityId: 'https://sp.example.com',
      samlSignRequests: true,
      samlAllowIdpInitiated: true,
      samlUsernameAttr: 'user',
      samlEmailAttr: 'mail',
      groupsClaim: 'memberOf',
      allowedGroups: 'admins, operators',
      allowedDomains: 'example.com',
      allowedEmails: 'u@example.com',
      groupRoleMappings: 'admins=admin',
    });
  });

  it('defaults displayName to "" and priority to 0 when they are absent', () => {
    const form = mapProviderDetailsToForm({
      id: 'p',
      name: 'P',
      type: 'oidc',
      enabled: false,
    });
    expect(form.displayName).toBe('');
    expect(form.priority).toBe(0);
    expect(form.enabled).toBe(false);
  });

  it('defaults the saml emailAttr to "email" when saml is present without emailAttr', () => {
    const form = mapProviderDetailsToForm({
      id: 'p',
      name: 'P',
      type: 'saml',
      enabled: true,
      saml: { signRequests: false },
    });
    expect(form.samlEmailAttr).toBe('email');
    expect(form.samlSignRequests).toBe(false);
    expect(form.samlAllowIdpInitiated).toBe(false);
  });

  it('defaults the saml emailAttr to "email" when saml is entirely absent (oidc provider)', () => {
    const form = mapProviderDetailsToForm({
      id: 'p',
      name: 'P',
      type: 'oidc',
      enabled: true,
    });
    expect(form.samlEmailAttr).toBe('email');
    expect(form.samlSpEntityId).toBe('');
  });

  it('falls back to the top-level groupsClaim when saml.groupsAttr is absent', () => {
    const form = mapProviderDetailsToForm({
      id: 'p',
      name: 'P',
      type: 'oidc',
      enabled: true,
      groupsClaim: 'roles',
    });
    expect(form.groupsClaim).toBe('roles');
  });

  it('defaults groupsClaim to "" when neither saml.groupsAttr nor top-level groupsClaim is set', () => {
    const form = mapProviderDetailsToForm({
      id: 'p',
      name: 'P',
      type: 'oidc',
      enabled: true,
    });
    expect(form.groupsClaim).toBe('');
  });

  it('defaults oidcScopes to "openid profile email" when scopes is an empty array (join -> "")', () => {
    const form = mapProviderDetailsToForm({
      id: 'p',
      name: 'P',
      type: 'oidc',
      enabled: true,
      oidc: { scopes: [] },
    });
    expect(form.oidcScopes).toBe('openid profile email');
  });

  it('joins a populated oidc scopes array with single spaces', () => {
    const form = mapProviderDetailsToForm({
      id: 'p',
      name: 'P',
      type: 'oidc',
      enabled: true,
      oidc: { scopes: ['openid', 'groups', 'custom_claim'] },
    });
    expect(form.oidcScopes).toBe('openid groups custom_claim');
  });

  it('serializes a populated oidc block (issuer/clientId/redirect/logout) into the form', () => {
    const form = mapProviderDetailsToForm({
      id: 'p',
      name: 'P',
      type: 'oidc',
      enabled: true,
      oidc: {
        issuerUrl: 'https://idp.example.com',
        clientId: 'pulse',
        redirectUrl: 'https://app.example.com/cb',
        logoutUrl: 'https://idp.example.com/logout',
      },
    });
    expect(form.oidcIssuerUrl).toBe('https://idp.example.com');
    expect(form.oidcClientId).toBe('pulse');
    expect(form.oidcRedirectUrl).toBe('https://app.example.com/cb');
    expect(form.oidcLogoutUrl).toBe('https://idp.example.com/logout');
    expect(form.oidcClientSecret).toBe('');
  });

  it('renders allowedGroups/Domains/Emails as "" for both absent and empty-array inputs', () => {
    // Both arms of listToString (`values && values.length > 0`) collapse to ''.
    const absent = mapProviderDetailsToForm({
      id: 'p',
      name: 'P',
      type: 'oidc',
      enabled: true,
    });
    expect(absent.allowedGroups).toBe('');
    expect(absent.allowedDomains).toBe('');
    expect(absent.allowedEmails).toBe('');

    const emptyArrays = mapProviderDetailsToForm({
      id: 'p',
      name: 'P',
      type: 'oidc',
      enabled: true,
      allowedGroups: [],
      allowedDomains: [],
      allowedEmails: [],
    });
    expect(emptyArrays.allowedGroups).toBe('');
    expect(emptyArrays.allowedDomains).toBe('');
    expect(emptyArrays.allowedEmails).toBe('');
  });
});

// ---- buildProviderPayload ---------------------------------------------------

describe('buildProviderPayload', () => {
  it('emits a fully-populated oidc payload including identity-restricted fields', () => {
    const payload = buildProviderPayload(
      oidcForm({
        id: 'oidc-1',
        name: '  Corp OIDC  ',
        displayName: '  Corp  ',
        priority: 3,
        oidcIssuerUrl: 'https://idp.example.com',
        oidcClientId: 'pulse',
        oidcClientSecret: 'shh',
        oidcRedirectUrl: 'https://app.example.com/cb',
        oidcLogoutUrl: 'https://idp.example.com/logout',
        oidcScopes: 'openid profile email groups',
        groupsClaim: 'groups',
        allowedGroups: 'admins, operators',
        allowedDomains: 'example.com',
        allowedEmails: 'u@example.com',
        groupRoleMappings: 'admins=admin, operators=operator',
      }),
    );

    expect(payload).toStrictEqual({
      id: 'oidc-1',
      name: 'Corp OIDC',
      type: 'oidc',
      enabled: true,
      displayName: 'Corp',
      priority: 3,
      allowedGroups: ['admins', 'operators'],
      allowedDomains: ['example.com'],
      allowedEmails: ['u@example.com'],
      groupRoleMappings: { admins: 'admin', operators: 'operator' },
      oidc: {
        issuerUrl: 'https://idp.example.com',
        clientId: 'pulse',
        clientSecret: 'shh',
        redirectUrl: 'https://app.example.com/cb',
        logoutUrl: 'https://idp.example.com/logout',
        scopes: ['openid', 'profile', 'email', 'groups'],
      },
      groupsClaim: 'groups',
    });
  });

  it('collapses every blank oidc field to undefined in the oidc arm', () => {
    // Default empty form, but force scopes to '' so splitList yields [].
    const payload = buildProviderPayload({ ...oidcForm(), oidcScopes: '' });

    expect(payload).toStrictEqual({
      id: undefined,
      name: '',
      type: 'oidc',
      enabled: true,
      displayName: undefined,
      priority: 0,
      allowedGroups: [],
      allowedDomains: [],
      allowedEmails: [],
      groupRoleMappings: {},
      oidc: {
        issuerUrl: '',
        clientId: '',
        clientSecret: undefined,
        redirectUrl: undefined,
        logoutUrl: undefined,
        scopes: [],
      },
      groupsClaim: undefined,
    });
  });

  it('emits a fully-populated saml payload (keeps groupsClaim aligned with groupsAttr)', () => {
    const payload = buildProviderPayload(
      samlForm({
        id: 'saml-1',
        name: '  Corp SAML  ',
        displayName: '  Corp  ',
        priority: 5,
        samlIdpMetadataUrl: 'https://idp.example.com/metadata',
        samlIdpMetadataXml: '<EntityDescriptor/>',
        samlIdpSsoUrl: 'https://idp.example.com/sso',
        samlIdpEntityId: 'https://idp.example.com',
        samlIdpCertificate: 'cert-data',
        samlSpEntityId: 'https://sp.example.com',
        samlSignRequests: true,
        samlAllowIdpInitiated: true,
        samlUsernameAttr: 'user',
        samlEmailAttr: 'mail',
        groupsClaim: 'memberOf',
        allowedGroups: 'admins',
        allowedDomains: 'example.com',
        allowedEmails: 'u@example.com',
        groupRoleMappings: 'admins=admin',
      }),
    );

    expect(payload).toStrictEqual({
      id: 'saml-1',
      name: 'Corp SAML',
      type: 'saml',
      enabled: true,
      displayName: 'Corp',
      priority: 5,
      allowedGroups: ['admins'],
      allowedDomains: ['example.com'],
      allowedEmails: ['u@example.com'],
      groupRoleMappings: { admins: 'admin' },
      saml: {
        idpMetadataUrl: 'https://idp.example.com/metadata',
        idpMetadataXml: '<EntityDescriptor/>',
        idpSsoUrl: 'https://idp.example.com/sso',
        idpEntityId: 'https://idp.example.com',
        idpCertificate: 'cert-data',
        spEntityId: 'https://sp.example.com',
        signRequests: true,
        allowIdpInitiated: true,
        usernameAttr: 'user',
        emailAttr: 'mail',
        groupsAttr: 'memberOf',
      },
      groupsClaim: 'memberOf',
    });
  });

  it('collapses every blank saml field to undefined in the saml arm', () => {
    const payload = buildProviderPayload(samlForm());

    expect(payload).toStrictEqual({
      id: undefined,
      name: '',
      type: 'saml',
      enabled: true,
      displayName: undefined,
      priority: 0,
      allowedGroups: [],
      allowedDomains: [],
      allowedEmails: [],
      groupRoleMappings: {},
      saml: {
        idpMetadataUrl: undefined,
        idpMetadataXml: undefined,
        idpSsoUrl: undefined,
        idpEntityId: undefined,
        idpCertificate: undefined,
        spEntityId: undefined,
        signRequests: false,
        allowIdpInitiated: false,
        usernameAttr: undefined,
        // The empty form seeds samlEmailAttr with 'email' (createEmptyProviderForm),
        // so it survives the `|| undefined` collapse.
        emailAttr: 'email',
        groupsAttr: undefined,
      },
      groupsClaim: undefined,
    });
  });
});

// ---- canTestProviderForm ----------------------------------------------------

describe('canTestProviderForm', () => {
  it('returns true for an oidc form with a populated issuerUrl', () => {
    expect(canTestProviderForm(oidcForm({ oidcIssuerUrl: 'https://idp.example.com' }))).toBe(true);
  });

  it('returns false for an oidc form with an empty issuerUrl', () => {
    expect(canTestProviderForm(oidcForm({ oidcIssuerUrl: '' }))).toBe(false);
  });

  it('returns false for an oidc form with a whitespace-only issuerUrl (trims to empty)', () => {
    expect(canTestProviderForm(oidcForm({ oidcIssuerUrl: '   ' }))).toBe(false);
  });

  it('returns true for a saml form with a populated metadataUrl', () => {
    expect(
      canTestProviderForm(samlForm({ samlIdpMetadataUrl: 'https://idp.example.com/metadata' })),
    ).toBe(true);
  });

  it('returns true for a saml form with only a populated metadataXml (metadataUrl absent)', () => {
    expect(canTestProviderForm(samlForm({ samlIdpMetadataXml: '<EntityDescriptor/>' }))).toBe(true);
  });

  it('returns true for a saml form with only a populated idpSsoUrl (first two blank)', () => {
    expect(canTestProviderForm(samlForm({ samlIdpSsoUrl: 'https://idp.example.com/sso' }))).toBe(
      true,
    );
  });

  it('returns false for a saml form with every test field blank', () => {
    expect(canTestProviderForm(samlForm())).toBe(false);
  });

  it('returns false for a saml form with every test field whitespace-only (trims to empty)', () => {
    expect(
      canTestProviderForm(
        samlForm({
          samlIdpMetadataUrl: '   ',
          samlIdpMetadataXml: '   ',
          samlIdpSsoUrl: '   ',
        }),
      ),
    ).toBe(false);
  });
});
