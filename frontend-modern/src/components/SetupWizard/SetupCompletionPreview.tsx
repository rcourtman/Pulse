import { Component } from 'solid-js';
import { useNavigate } from '@solidjs/router';
import { SetupCompletionPanel } from './SetupCompletionPanel';
import type { WizardState } from './SetupWizard';

const previewWizardState: WizardState = {
  username: 'admin',
  password: 'preview-password',
  apiToken: 'preview-install-token',
};

export const SetupCompletionPreview: Component = () => {
  const navigate = useNavigate();

  return (
    <SetupCompletionPanel
      state={previewWizardState}
      onComplete={(nextPath) => {
        if (nextPath) {
          navigate(nextPath);
        }
      }}
    />
  );
};
