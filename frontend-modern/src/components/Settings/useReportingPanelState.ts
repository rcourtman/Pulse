import { createEffect, createSignal, onMount } from 'solid-js';
import { apiFetch } from '@/utils/apiClient';
import { showSuccess, showWarning } from '@/utils/toast';
import type { SelectedResource } from '@/components/Settings/ResourcePicker';
import {
  entitlements,
  getUpgradeActionUrlOrFallback,
  hasFeature,
  licenseLoaded,
  loadLicenseStatus,
} from '@/stores/license';
import { trackPaywallViewed } from '@/utils/upgradeMetrics';
import {
  getReportingGenerateErrorMessage,
  getReportingGenerateSelectionRequiredMessage,
  getReportingGenerateSuccessMessage,
  getReportingInventoryExportErrorMessage,
  getReportingInventoryExportSuccessMessage,
} from '@/utils/reportingPresentation';
import { runStartProTrialAction } from '@/utils/trialStartAction';
import {
  buildReportingRequest,
  getReportingRangeStart,
  type ReportingFormat,
  type ReportingRangeValue,
} from '@/components/Settings/reportingPanelModel';
import {
  buildVMInventoryExportDefinitionRequest,
  buildVMInventoryExportRequest,
  parseVMInventoryExportDefinition,
  type ReportingInventoryExportDefinition,
} from '@/components/Settings/reportingInventoryExportModel';

export const useReportingPanelState = () => {
  const [selectedResources, setSelectedResources] = createSignal<SelectedResource[]>([]);
  const [metricType, setMetricType] = createSignal('');
  const [format, setFormat] = createSignal<ReportingFormat>('pdf');
  const [range, setRange] = createSignal<ReportingRangeValue>('24h');
  const [generating, setGenerating] = createSignal(false);
  const [exportingInventory, setExportingInventory] = createSignal(false);
  const [inventoryDefinition, setInventoryDefinition] =
    createSignal<ReportingInventoryExportDefinition | null>(null);
  const [inventoryDefinitionLoading, setInventoryDefinitionLoading] = createSignal(false);
  const [inventoryDefinitionError, setInventoryDefinitionError] = createSignal('');
  const [inventoryDefinitionRequested, setInventoryDefinitionRequested] = createSignal(false);
  const [title, setTitle] = createSignal('');
  const [startingTrial, setStartingTrial] = createSignal(false);

  const isLocked = () => licenseLoaded() && !hasFeature('advanced_reporting');
  const canStartTrial = () => entitlements()?.trial_eligible !== false;
  const isReportingEnabled = () => licenseLoaded() && hasFeature('advanced_reporting');
  const upgradeActionUrl = () => getUpgradeActionUrlOrFallback('advanced_reporting');

  onMount(() => {
    loadLicenseStatus();
  });

  createEffect((wasVisible: boolean) => {
    const visible = isLocked();
    if (visible && !wasVisible) {
      trackPaywallViewed('advanced_reporting', 'settings_reporting_panel');
    }
    return visible;
  }, false);

  createEffect(() => {
    if (
      !isReportingEnabled() ||
      inventoryDefinition() ||
      inventoryDefinitionLoading() ||
      inventoryDefinitionRequested()
    ) {
      return;
    }

    void (async () => {
      setInventoryDefinitionRequested(true);
      setInventoryDefinitionLoading(true);
      setInventoryDefinitionError('');
      try {
        const request = buildVMInventoryExportDefinitionRequest();
        const response = await apiFetch(request.url);
        if (!response.ok) {
          const text = await response.text();
          throw new Error(text || getReportingInventoryExportErrorMessage());
        }

        setInventoryDefinition(parseVMInventoryExportDefinition(await response.json()));
      } catch (error) {
        console.error('VM inventory export definition error:', error);
        setInventoryDefinitionError(
          error instanceof Error ? error.message : getReportingInventoryExportErrorMessage(),
        );
      } finally {
        setInventoryDefinitionLoading(false);
      }
    })();
  });

  const handleStartTrial = async () => {
    if (startingTrial()) return;
    setStartingTrial(true);
    try {
      await runStartProTrialAction({
        showSuccess,
        showError: showWarning,
      });
    } finally {
      setStartingTrial(false);
    }
  };

  const downloadResponseBlob = async (filename: string, response: Response) => {
    const blob = await response.blob();
    const url = window.URL.createObjectURL(blob);
    const anchor = document.createElement('a');
    anchor.href = url;
    anchor.download = filename;
    document.body.appendChild(anchor);
    anchor.click();
    window.URL.revokeObjectURL(url);
    document.body.removeChild(anchor);
  };

  const handleGenerate = async () => {
    const resources = selectedResources();
    if (resources.length === 0) {
      showWarning(getReportingGenerateSelectionRequiredMessage());
      return;
    }

    setGenerating(true);
    try {
      const now = new Date();
      const start = getReportingRangeStart(range(), now);
      const request = buildReportingRequest({
        end: now.toISOString(),
        format: format(),
        metricType: metricType(),
        now,
        resources,
        start: start.toISOString(),
        title: title(),
      });

      const response = await apiFetch(request.request.url, request.request.init);
      if (!response.ok) {
        const text = await response.text();
        throw new Error(text || getReportingGenerateErrorMessage());
      }

      await downloadResponseBlob(request.filename, response);

      showSuccess(getReportingGenerateSuccessMessage());
    } catch (error) {
      console.error('Report generation error:', error);
      showWarning(error instanceof Error ? error.message : getReportingGenerateErrorMessage());
    } finally {
      setGenerating(false);
    }
  };

  const handleExportVMInventory = async () => {
    if (exportingInventory()) return;

    setExportingInventory(true);
    try {
      const request = buildVMInventoryExportRequest(new Date(), inventoryDefinition());
      const response = await apiFetch(request.request.url);
      if (!response.ok) {
        const text = await response.text();
        throw new Error(text || getReportingInventoryExportErrorMessage());
      }

      await downloadResponseBlob(request.filename, response);
      showSuccess(getReportingInventoryExportSuccessMessage());
    } catch (error) {
      console.error('VM inventory export error:', error);
      showWarning(
        error instanceof Error ? error.message : getReportingInventoryExportErrorMessage(),
      );
    } finally {
      setExportingInventory(false);
    }
  };

  return {
    canStartTrial,
    exportingInventory,
    format,
    handleExportVMInventory,
    generating,
    handleGenerate,
    handleStartTrial,
    inventoryDefinition,
    inventoryDefinitionError,
    inventoryDefinitionLoading,
    isLocked,
    isReportingEnabled,
    metricType,
    range,
    selectedResources,
    setFormat,
    setMetricType,
    setRange,
    setSelectedResources,
    setTitle,
    startingTrial,
    title,
    upgradeActionUrl,
  };
};
