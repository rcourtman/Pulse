import { Component, createMemo } from 'solid-js';
import {
  AIModelPicker,
  type AIModelPickerExtraOption,
  type AIModelPickerModelSection,
} from '@/components/shared/AIModelPicker';
import { formatAIModelRouteLabel } from '@/utils/aiProviderPresentation';
import type { ModelInfo } from './types';

export interface ModelSelectorProps {
  models: ModelInfo[];
  selectedModel: string;
  defaultModel?: string;
  defaultModelLabel?: string;
  chatOverrideModel?: string;
  chatOverrideLabel?: string;
  recentModelIds?: string[];
  isLoading?: boolean;
  error?: string;
  openRequest?: number;
  initialSearchQuery?: string;
  onModelSelect: (modelId: string) => void;
  onRefresh?: () => void;
}

/**
 * Chat-specific model selector wrapper.
 *
 * OpenCode's DialogModel keeps recent models above the provider catalog while
 * the shared picker owns filtering, route labels, current-selection visibility,
 * and dropdown mechanics.
 */
export const ModelSelector: Component<ModelSelectorProps> = (props) => {
  const normalizedChatOverrideModel = createMemo(() => props.chatOverrideModel?.trim() || '');
  const shouldShowChatOverride = createMemo(() => {
    const override = normalizedChatOverrideModel();
    if (!override) return false;
    const selected = props.selectedModel?.trim() || '';
    if (selected && selected === override) return false;
    const defaultModel = props.defaultModel?.trim() || '';
    if (defaultModel && defaultModel === override) return false;
    return true;
  });

  const defaultOption = createMemo(() => ({
    label: 'Default',
    description: props.defaultModelLabel
      ? `Use configured default model (${props.defaultModelLabel})`
      : 'Use configured default model',
  }));

  const extraOptions = createMemo<AIModelPickerExtraOption[]>(() => {
    if (!shouldShowChatOverride()) return [];
    const override = normalizedChatOverrideModel();
    return [
      {
        id: override,
        label: 'Chat override',
        description: props.chatOverrideLabel || formatAIModelRouteLabel(override),
      },
    ];
  });

  const modelSections = createMemo<AIModelPickerModelSection[]>(() => {
    const recentModelIds = props.recentModelIds || [];
    if (recentModelIds.length === 0) return [];
    return [
      {
        title: 'Recent',
        modelIds: recentModelIds,
      },
    ];
  });

  const selectionBadge = createMemo(() => {
    if ((props.selectedModel || '').trim()) return '';
    return props.defaultModelLabel ? 'default' : '';
  });

  return (
    <AIModelPicker
      models={props.models}
      selectedModel={props.selectedModel}
      onModelSelect={props.onModelSelect}
      defaultOption={defaultOption()}
      extraOptions={extraOptions()}
      modelSections={modelSections()}
      emptySelectionLabel={props.defaultModelLabel || 'Default'}
      selectionBadge={selectionBadge()}
      title="Select model for this chat"
      customModelDescription="Custom provider:model route"
      isLoading={props.isLoading}
      error={props.error}
      openRequest={props.openRequest}
      initialSearchQuery={props.initialSearchQuery}
      onRefresh={props.onRefresh}
      align="left"
      buttonClass="flex flex-shrink-0 items-center gap-1.5 rounded-md border border-border bg-surface px-2.5 py-1.5 text-[11px] text-muted transition-colors hover:border-border hover:text-base-content"
      buttonLabelClass="max-w-[90px] truncate font-medium sm:max-w-[180px]"
      dropdownClass="w-80 max-w-[calc(100vw-2rem)]"
    />
  );
};
