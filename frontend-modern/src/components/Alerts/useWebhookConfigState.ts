import { createEffect, createSignal, onCleanup } from 'solid-js';
import type { Accessor } from 'solid-js';

import { NotificationsAPI, type Webhook, type WebhookTemplate } from '@/api/notifications';
import { logger } from '@/utils/logger';
import {
  getAlertWebhookCustomFieldInputs,
  normalizeAlertWebhookCustomFields,
} from '@/utils/alertWebhookPresentation';

import type { WebhookConfigProps } from './WebhookConfig';

export type HeaderInput = {
  id: string;
  key: string;
  value: string;
};

export type CustomFieldInput = HeaderInput & {
  label?: string;
  placeholder?: string;
  required?: boolean;
};

export type WebhookConfigFormData = Omit<Webhook, 'id'> & {
  service: string;
  payloadTemplate?: string;
};

type SetterLike<T> = (value: T | ((prev: T) => T)) => T;

const createDefaultFormData = (): WebhookConfigFormData => ({
  name: '',
  url: '',
  method: 'POST',
  service: 'generic',
  headers: { 'Content-Type': 'application/json' },
  enabled: true,
  payloadTemplate: '',
  customFields: {},
  mention: '',
  tagFilter: [],
  tagFilterMode: 'all',
  minimumSeverity: 'all',
});

const buildMapFromInputs = (
  inputs: Array<{ key: string; value: string }>,
): Record<string, string> => {
  const map: Record<string, string> = {};
  inputs.forEach(({ key, value }) => {
    if (key) {
      map[key] = value;
    }
  });
  return map;
};

const createHeaderInput = (index: number, key = '', value = ''): HeaderInput => ({
  id: `header-${Date.now()}-${index}`,
  key,
  value,
});

const createCustomFieldInput = (
  field: Omit<CustomFieldInput, 'id'>,
  index: number,
): CustomFieldInput => ({
  id: `custom-${field.key || 'field'}-${Date.now()}-${index}`,
  ...field,
});

export interface WebhookConfigState {
  adding: Accessor<boolean>;
  saving: Accessor<boolean>;
  saveError: Accessor<string | null>;
  mutationError: Accessor<string | null>;
  editingId: Accessor<string | null>;
  formData: Accessor<WebhookConfigFormData>;
  templates: Accessor<WebhookTemplate[]>;
  currentTemplate: Accessor<WebhookTemplate | undefined>;
  showServiceDropdown: Accessor<boolean>;
  headerInputs: Accessor<HeaderInput[]>;
  customFieldInputs: Accessor<CustomFieldInput[]>;
  allEnabled: Accessor<boolean>;
  someEnabled: Accessor<boolean>;
  setFormData: SetterLike<WebhookConfigFormData>;
  setShowServiceDropdown: SetterLike<boolean>;
  openAddForm: () => void;
  cancelForm: () => void;
  editWebhook: (webhook: Webhook) => void;
  selectService: (service: string) => void;
  saveWebhook: () => Promise<void>;
  testWebhookForm: () => void;
  toggleAllWebhooks: (enabled: boolean) => Promise<void>;
  toggleWebhook: (webhook: Webhook) => Promise<void>;
  deleteWebhook: (webhook: Webhook) => Promise<void>;
  updateHeaderInput: (index: number, patch: Partial<HeaderInput>) => void;
  removeHeaderInput: (index: number) => void;
  addHeaderInput: () => void;
  updateCustomFieldInput: (index: number, patch: Partial<CustomFieldInput>) => void;
  removeCustomFieldInput: (index: number) => void;
  addCustomFieldInput: () => void;
}

export function useWebhookConfigState(props: WebhookConfigProps): WebhookConfigState {
  const [adding, setAdding] = createSignal(false);
  const [saving, setSaving] = createSignal(false);
  const [saveError, setSaveError] = createSignal<string | null>(null);
  const [mutationError, setMutationError] = createSignal<string | null>(null);
  const [editingId, setEditingId] = createSignal<string | null>(null);
  const [formData, setFormData] = createSignal<WebhookConfigFormData>(createDefaultFormData());
  const [templates, setTemplates] = createSignal<WebhookTemplate[]>([]);
  const [showServiceDropdown, setShowServiceDropdown] = createSignal(false);
  const [headerInputs, setHeaderInputs] = createSignal<HeaderInput[]>([]);
  const [customFieldInputs, _setCustomFieldInputs] = createSignal<CustomFieldInput[]>([]);
  let disposed = false;
  onCleanup(() => {
    disposed = true;
  });

  const setCustomFieldInputs = (inputs: CustomFieldInput[]) => {
    _setCustomFieldInputs(inputs);
    setFormData((prev) => ({
      ...prev,
      customFields: buildMapFromInputs(inputs),
    }));
  };

  const updateCustomFieldInputs = (updater: (inputs: CustomFieldInput[]) => CustomFieldInput[]) => {
    _setCustomFieldInputs((prev) => {
      const next = updater(prev);
      setFormData((prevForm) => ({
        ...prevForm,
        customFields: buildMapFromInputs(next),
      }));
      return next;
    });
  };

  const resetForm = () => {
    setSaveError(null);
    setAdding(false);
    setEditingId(null);
    setFormData(createDefaultFormData());
    setHeaderInputs([]);
    setCustomFieldInputs([]);
    setShowServiceDropdown(false);
  };

  createEffect(async () => {
    try {
      const data = await NotificationsAPI.getWebhookTemplates();
      setTemplates(data);
    } catch (err) {
      logger.error('Failed to load webhook templates:', err);
    }
  });

  const currentTemplate = () =>
    templates().find((template) => template.service === formData().service);

  // The form and list share one writer. Do not let a late toggle acknowledgement
  // replace a later operator choice, or let list changes invalidate an open draft.
  const mutateList = async (mutate: () => boolean | Promise<boolean>) => {
    if (disposed || saving() || adding()) return;
    setSaving(true);
    setMutationError(null);
    const failureMessage =
      'Could not confirm all webhook changes. Some destinations may already have changed. Check the configured destinations before trying again.';
    try {
      if ((await mutate()) !== true && !disposed) setMutationError(failureMessage);
    } catch {
      // Callback/provider text may contain private destination details.
      if (!disposed) setMutationError(failureMessage);
    } finally {
      if (!disposed) setSaving(false);
    }
  };

  const toggleAllWebhooks = (enabled: boolean) =>
    mutateList(async () => {
      // Capture only changed rows before yielding. Each acknowledgement updates
      // inventory; a failure leaves earlier accepted changes intact, not rolled back.
      const submitted = props.webhooks
        .filter((webhook) => webhook.enabled !== enabled)
        .map((webhook) => structuredClone({ ...webhook, enabled }));
      for (const webhook of submitted) {
        if (disposed) return false;
        if ((await props.onUpdate(webhook)) !== true) return false;
      }
      return true;
    });

  const toggleWebhook = (webhook: Webhook) =>
    mutateList(() => props.onUpdate(structuredClone({ ...webhook, enabled: !webhook.enabled })));

  const deleteWebhook = async (webhook: Webhook) => {
    if (!webhook.id) return;
    await mutateList(() => props.onDelete(webhook.id));
  };

  const allEnabled = () => props.webhooks.every((webhook) => webhook.enabled);
  const someEnabled = () => props.webhooks.some((webhook) => webhook.enabled);

  const openAddForm = () => {
    if (disposed || saving()) return;
    setSaveError(null);
    setAdding(true);
    setEditingId(null);
    setFormData(createDefaultFormData());
    // A new webhook starts by choosing the service: most installs want
    // Discord, Telegram, ntfy and the like, not the generic JSON form.
    setShowServiceDropdown(true);
    setHeaderInputs([createHeaderInput(0, 'Content-Type', 'application/json')]);
    setCustomFieldInputs([]);
  };

  const cancelForm = () => {
    if (disposed || saving()) return;
    resetForm();
  };

  const editWebhook = (webhook: Webhook) => {
    if (disposed || saving()) return;
    setSaveError(null);
    if (webhook.id) {
      setEditingId(webhook.id);
    }

    setFormData({
      ...webhook,
      service: webhook.service || 'generic',
      payloadTemplate: webhook.template || '',
      customFields: normalizeAlertWebhookCustomFields(
        webhook.service || 'generic',
        webhook.customFields || {},
      ),
      mention: webhook.mention || '',
      tagFilter: webhook.tagFilter ?? [],
      tagFilterMode: webhook.tagFilterMode === 'any' ? 'any' : 'all',
      minimumSeverity:
        webhook.minimumSeverity === 'critical' || webhook.minimumSeverity === 'warning'
          ? webhook.minimumSeverity
          : 'all',
    });

    const headers = webhook.headers || {};
    setHeaderInputs(
      Object.entries(headers).map(([key, value], index) => createHeaderInput(index, key, value)),
    );

    const service = webhook.service || 'generic';
    const existingCustomFields = webhook.customFields || {};
    setCustomFieldInputs(
      getAlertWebhookCustomFieldInputs(service, existingCustomFields).map((field, index) =>
        createCustomFieldInput(field, index),
      ),
    );

    setShowServiceDropdown(false);
    setAdding(true);
  };

  const selectService = (service: string) => {
    const template = templates().find((candidate) => candidate.service === service);

    if (template) {
      setFormData((prev) => ({
        ...prev,
        service: template.service,
        method: template.method,
        headers: { ...template.headers },
        name: prev.name || template.name,
        payloadTemplate: service === 'generic' ? prev.payloadTemplate : '',
      }));

      const headers = template.headers || {};
      setHeaderInputs(
        Object.entries(headers).map(([key, value], index) => createHeaderInput(index, key, value)),
      );
    } else {
      setFormData((prev) => ({
        ...prev,
        service,
      }));
    }

    setCustomFieldInputs(
      getAlertWebhookCustomFieldInputs(service, formData().customFields || {}).map((field, index) =>
        createCustomFieldInput(field, index),
      ),
    );
    setShowServiceDropdown(false);
  };

  const saveWebhook = async () => {
    if (disposed || saving()) return;
    const data = formData();
    if (!data.name || !data.url) return;

    const headers = buildMapFromInputs(headerInputs());
    const customFields = buildMapFromInputs(customFieldInputs());
    const normalizedCustomFields = normalizeAlertWebhookCustomFields(data.service, customFields);

    setSaving(true);
    setSaveError(null);
    // The owner updates inventory only after an acknowledged API write. Do
    // not discard this draft (including masked saved fields) on an unconfirmed
    // result, and never retry a potentially accepted create automatically.
    const failureMessage =
      'Could not confirm the save. Your changes are still here. Check the configured destinations in another tab before trying again.';
    try {
      const accepted = editingId()
        ? await props.onUpdate({
            ...data,
            id: editingId()!,
            headers,
            service: data.service,
            template: data.payloadTemplate,
            customFields: normalizedCustomFields,
            mention: data.mention,
          })
        : await props.onAdd({
            name: data.name,
            url: data.url,
            method: data.method,
            headers,
            enabled: data.enabled,
            service: data.service,
            template: data.payloadTemplate,
            customFields: normalizedCustomFields,
            mention: data.mention,
            tagFilter: data.tagFilter ?? [],
            tagFilterMode: data.tagFilterMode === 'any' ? 'any' : 'all',
            minimumSeverity:
              data.minimumSeverity === 'critical' || data.minimumSeverity === 'warning'
                ? data.minimumSeverity
                : 'all',
          });
      if (!disposed) {
        if (accepted === true) resetForm();
        else setSaveError(failureMessage);
      }
    } catch {
      // Callback/provider text can contain submitted destination details.
      if (!disposed) setSaveError(failureMessage);
    } finally {
      if (!disposed) setSaving(false);
    }
  };

  const testWebhookForm = () => {
    const data = formData();
    const headers = buildMapFromInputs(headerInputs());
    const customFields = buildMapFromInputs(customFieldInputs());
    const { payloadTemplate, ...restFormData } = data;
    const testPayload = {
      ...restFormData,
      headers,
      customFields: normalizeAlertWebhookCustomFields(data.service, customFields),
      template: payloadTemplate ?? restFormData.template ?? '',
    };
    const tempId = editingId() || 'temp-new-webhook';
    props.onTest(tempId, testPayload);
  };

  const updateHeaderInput = (index: number, patch: Partial<HeaderInput>) => {
    setHeaderInputs((inputs) => {
      const next = [...inputs];
      next[index] = { ...next[index], ...patch };
      return next;
    });
  };

  const removeHeaderInput = (index: number) => {
    setHeaderInputs((inputs) => inputs.filter((_, currentIndex) => currentIndex !== index));
  };

  const addHeaderInput = () => {
    setHeaderInputs((inputs) => [...inputs, createHeaderInput(inputs.length)]);
  };

  const updateCustomFieldInput = (index: number, patch: Partial<CustomFieldInput>) => {
    updateCustomFieldInputs((inputs) => {
      const next = [...inputs];
      next[index] = { ...next[index], ...patch };
      return next;
    });
  };

  const removeCustomFieldInput = (index: number) => {
    updateCustomFieldInputs((inputs) => inputs.filter((_, currentIndex) => currentIndex !== index));
  };

  const addCustomFieldInput = () => {
    updateCustomFieldInputs((inputs) => [
      ...inputs,
      createCustomFieldInput({ key: '', value: '' }, inputs.length),
    ]);
  };

  return {
    adding,
    saving,
    saveError,
    mutationError,
    editingId,
    formData,
    templates,
    currentTemplate,
    showServiceDropdown,
    headerInputs,
    customFieldInputs,
    allEnabled,
    someEnabled,
    setFormData,
    setShowServiceDropdown,
    openAddForm,
    cancelForm,
    editWebhook,
    selectService,
    saveWebhook,
    testWebhookForm,
    toggleAllWebhooks,
    toggleWebhook,
    deleteWebhook,
    updateHeaderInput,
    removeHeaderInput,
    addHeaderInput,
    updateCustomFieldInput,
    removeCustomFieldInput,
    addCustomFieldInput,
  };
}
