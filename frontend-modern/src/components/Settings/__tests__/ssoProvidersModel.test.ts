import { describe, expect, it } from 'vitest';
import {
  buildProviderPayload,
  createEmptyProviderForm,
  mapProviderDetailsToForm,
} from '../ssoProvidersModel';

describe('ssoProvidersModel', () => {
  it('serializes OIDC group claim settings for restrictions and role mappings', () => {
    const form = {
      ...createEmptyProviderForm(),
      name: 'Corporate OIDC',
      type: 'oidc' as const,
      oidcIssuerUrl: 'https://idp.example.com',
      oidcClientId: 'pulse',
      groupsClaim: 'groups',
      allowedGroups: 'admins, operators',
      groupRoleMappings: 'admins=admin, operators=operator',
    };

    const payload = buildProviderPayload(form);

    expect(payload.groupsClaim).toBe('groups');
    expect(payload.allowedGroups).toEqual(['admins', 'operators']);
    expect(payload.groupRoleMappings).toEqual({
      admins: 'admin',
      operators: 'operator',
    });
    expect(payload.oidc).toMatchObject({
      issuerUrl: 'https://idp.example.com',
      clientId: 'pulse',
    });
  });

  it('maps OIDC provider details back to the shared groups claim field', () => {
    const form = mapProviderDetailsToForm({
      id: 'corp-oidc',
      name: 'Corporate OIDC',
      type: 'oidc',
      enabled: true,
      groupsClaim: 'roles',
      allowedGroups: ['admins'],
      groupRoleMappings: {
        admins: 'admin',
      },
    });

    expect(form.groupsClaim).toBe('roles');
    expect(form.allowedGroups).toBe('admins');
    expect(form.groupRoleMappings).toBe('admins=admin');
  });

  it('keeps SAML provider and SAML protocol groups attributes aligned', () => {
    const form = {
      ...createEmptyProviderForm(),
      name: 'Corporate SAML',
      type: 'saml' as const,
      groupsClaim: 'memberOf',
      samlIdpSsoUrl: 'https://idp.example.com/sso',
    };

    const payload = buildProviderPayload(form);
    const saml = payload.saml as Record<string, unknown>;

    expect(payload.groupsClaim).toBe('memberOf');
    expect(saml.groupsAttr).toBe('memberOf');
  });
});
