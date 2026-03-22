import { createEffect, createMemo, createSignal } from 'solid-js';

import { NotificationsAPI, type EmailProvider } from '@/api/notifications';
import type { UIEmailConfig } from '@/features/alerts/types';
import { logger } from '@/utils/logger';

export interface EmailProviderSelectStateProps {
  config: UIEmailConfig;
  onChange: (config: UIEmailConfig) => void;
}

export function useEmailProviderSelectState(props: EmailProviderSelectStateProps) {
  const [providers, setProviders] = createSignal<EmailProvider[]>([]);
  const [showAdvanced, setShowAdvanced] = createSignal(false);
  const [showInstructions, setShowInstructions] = createSignal(false);

  const loadEmailProviders = async () => {
    try {
      const data = await NotificationsAPI.getEmailProviders();
      setProviders(data);
    } catch (err) {
      logger.error('Failed to load email providers', err);
    }
  };

  createEffect(() => {
    void loadEmailProviders();
  });

  const currentProvider = createMemo(() =>
    providers().find((provider) => provider.name === props.config.provider),
  );

  const applyProviderDefaults = (provider: EmailProvider | undefined) => {
    if (!provider) {
      props.onChange({ ...props.config, provider: '' });
      setShowInstructions(false);
      return;
    }

    props.onChange({
      ...props.config,
      provider: provider.name,
      server: provider.smtpHost,
      port: provider.smtpPort,
      tls: provider.tls,
      startTLS: provider.startTLS,
      username: provider.name === 'SendGrid' ? 'apikey' : props.config.username,
    });
    setShowInstructions(true);
  };

  const handleProviderChange = (value: string) => {
    if (!value) {
      applyProviderDefaults(undefined);
      return;
    }
    applyProviderDefaults(providers().find((provider) => provider.name === value));
  };

  const toggleShowAdvanced = () => {
    setShowAdvanced((value) => !value);
  };

  const toggleShowInstructions = () => {
    setShowInstructions((value) => !value);
  };

  return {
    providers,
    currentProvider,
    showAdvanced,
    showInstructions,
    handleProviderChange,
    applyProviderDefaults,
    toggleShowAdvanced,
    toggleShowInstructions,
  };
}
