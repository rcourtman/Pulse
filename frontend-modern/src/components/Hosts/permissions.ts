import { SETTINGS_WRITE_SCOPE } from '@/constants/apiScopes';

export const hasSettingsWriteAccess = (tokenScopes?: string[]): boolean => {
  if (!tokenScopes || tokenScopes.length === 0) {
    return true;
  }

  return tokenScopes.includes('*') || tokenScopes.includes(SETTINGS_WRITE_SCOPE);
};
