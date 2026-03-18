import type { Component } from 'solid-js';
import Globe from 'lucide-solid/icons/globe';
import Key from 'lucide-solid/icons/key';
import type { SSOProviderType } from '@/utils/ssoProviderPresentation';

interface SSOProviderTypeIconProps {
  type: SSOProviderType;
  class?: string;
}

export const SSOProviderTypeIcon: Component<SSOProviderTypeIconProps> = (props) => {
  return props.type === 'oidc' ? (
    <Globe class={props.class ?? 'w-5 h-5 text-muted'} />
  ) : (
    <Key class={props.class ?? 'w-5 h-5 text-muted'} />
  );
};
