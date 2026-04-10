import { Component, createMemo } from 'solid-js';
import { useLocation, useNavigate } from '@solidjs/router';
import { SetupCompletionPanel } from './SetupCompletionPanel';
import type { WizardState } from './SetupWizard';
import { getSetupCompletionPreviewScenario } from './setupCompletionPreviewScenarios';

const previewWizardState: WizardState = {
  username: 'admin',
  password: 'preview-password',
  apiToken: 'preview-install-token',
};

export const SetupCompletionPreview: Component = () => {
  const location = useLocation();
  const navigate = useNavigate();
  const previewScenario = createMemo(() =>
    getSetupCompletionPreviewScenario(location.search),
  );

  return (
    <SetupCompletionPanel
      connectedResourcesOverride={previewScenario().resources}
      state={previewWizardState}
      onComplete={(nextPath) => {
        if (nextPath) {
          navigate(nextPath);
        }
      }}
    />
  );
};
