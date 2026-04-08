import { createSignal } from 'solid-js';
import { AIAPI } from '@/api/ai';
import { eventBus } from '@/stores/events';
import type { AISettings, ModelInfo } from '@/types/ai';
import { logger } from '@/utils/logger';

const [aiRuntimeSettings, setAIRuntimeSettings] = createSignal<AISettings | null>(null);
const [aiRuntimeSettingsLoading, setAIRuntimeSettingsLoading] = createSignal(false);
const [aiRuntimeSettingsLoaded, setAIRuntimeSettingsLoaded] = createSignal(false);
const [aiRuntimeSettingsLoadError, setAIRuntimeSettingsLoadError] = createSignal<Error | null>(null);

const [aiRuntimeModels, setAIRuntimeModels] = createSignal<ModelInfo[]>([]);
const [aiRuntimeModelsLoading, setAIRuntimeModelsLoading] = createSignal(false);
const [aiRuntimeModelsLoaded, setAIRuntimeModelsLoaded] = createSignal(false);
const [aiRuntimeModelsError, setAIRuntimeModelsError] = createSignal('');

let inFlightSettingsLoad: Promise<AISettings | null> | null = null;
let inFlightModelsLoad: Promise<ModelInfo[]> | null = null;

function resetAIRuntimeState() {
  setAIRuntimeSettings(null);
  setAIRuntimeSettingsLoading(false);
  setAIRuntimeSettingsLoaded(false);
  setAIRuntimeSettingsLoadError(null);
  setAIRuntimeModels([]);
  setAIRuntimeModelsLoading(false);
  setAIRuntimeModelsLoaded(false);
  setAIRuntimeModelsError('');
  inFlightSettingsLoad = null;
  inFlightModelsLoad = null;
}

export function syncAIRuntimeSettings(next: AISettings | null) {
  setAIRuntimeSettings(next);
  setAIRuntimeSettingsLoadError(null);
  setAIRuntimeSettingsLoaded(true);
  setAIRuntimeSettingsLoading(false);
}

export function syncAIRuntimeModels(next: ModelInfo[], error = '') {
  setAIRuntimeModels(next);
  setAIRuntimeModelsError(error);
  setAIRuntimeModelsLoaded(true);
  setAIRuntimeModelsLoading(false);
}

export function clearAIRuntimeModels() {
  syncAIRuntimeModels([]);
}

export async function loadAIRuntimeSettings(force = false): Promise<AISettings | null> {
  if (inFlightSettingsLoad) {
    if (!force) {
      return inFlightSettingsLoad;
    }
    await inFlightSettingsLoad;
  }

  if (aiRuntimeSettingsLoaded() && !force) {
    return aiRuntimeSettings();
  }

  setAIRuntimeSettingsLoading(true);
  const request = (async () => {
    try {
      const next = await AIAPI.getSettings();
      syncAIRuntimeSettings(next);
      return next;
    } catch (error) {
      const normalized = error instanceof Error ? error : new Error(String(error));
      setAIRuntimeSettingsLoadError(normalized);
      setAIRuntimeSettingsLoaded(true);
      setAIRuntimeSettingsLoading(false);
      logger.debug('[aiRuntimeState] Failed to load AI settings', normalized);
      throw normalized;
    }
  })();

  inFlightSettingsLoad = request;
  try {
    return await request;
  } finally {
    if (inFlightSettingsLoad === request) {
      inFlightSettingsLoad = null;
    }
  }
}

export async function loadAIRuntimeModels(force = false): Promise<ModelInfo[]> {
  if (inFlightModelsLoad) {
    if (!force) {
      return inFlightModelsLoad;
    }
    await inFlightModelsLoad;
  }

  if (aiRuntimeModelsLoaded() && !force) {
    return aiRuntimeModels();
  }

  setAIRuntimeModelsLoading(true);
  setAIRuntimeModelsError('');
  const request = (async () => {
    try {
      const result = await AIAPI.getModels();
      syncAIRuntimeModels(result.models ?? [], result.error ?? '');
      return result.models ?? [];
    } catch (error) {
      const message = error instanceof Error ? error.message : 'Failed to load models.';
      setAIRuntimeModelsError(message);
      setAIRuntimeModels([]);
      setAIRuntimeModelsLoaded(true);
      setAIRuntimeModelsLoading(false);
      logger.debug('[aiRuntimeState] Failed to load AI models', error);
      throw error;
    }
  })();

  inFlightModelsLoad = request;
  try {
    return await request;
  } finally {
    if (inFlightModelsLoad === request) {
      inFlightModelsLoad = null;
    }
  }
}

eventBus.on('org_switched', () => {
  resetAIRuntimeState();
});

export {
  aiRuntimeModels,
  aiRuntimeModelsError,
  aiRuntimeModelsLoaded,
  aiRuntimeModelsLoading,
  aiRuntimeSettings,
  aiRuntimeSettingsLoaded,
  aiRuntimeSettingsLoading,
  aiRuntimeSettingsLoadError,
  resetAIRuntimeState,
};
