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
  getReportingCatalogErrorMessage,
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
  type ReportingRangeValue,
} from '@/components/Settings/reportingPanelModel';
import {
  buildVMInventoryExportRequest,
} from '@/components/Settings/reportingInventoryExportModel';
import {
  buildReportingCatalogRequest,
  parseReportingCatalog,
  type ReportingCatalog,
  type ReportingFormat,
} from '@/components/Settings/reportingCatalogModel';

export const useReportingPanelState = () => {
  const [selectedResources, setSelectedResources] = createSignal<SelectedResource[]>([]);
  const [metricType, setMetricType] = createSignal('');
  const [format, setFormat] = createSignal<ReportingFormat | null>(null);
  const [range, setRange] = createSignal<ReportingRangeValue | null>(null);
  const [generating, setGenerating] = createSignal(false);
  const [exportingInventory, setExportingInventory] = createSignal(false);
  const [reportingCatalog, setReportingCatalog] = createSignal<ReportingCatalog | null>(null);
  const [reportingCatalogLoading, setReportingCatalogLoading] = createSignal(false);
  const [reportingCatalogError, setReportingCatalogError] = createSignal('');
  const [reportingCatalogRequested, setReportingCatalogRequested] = createSignal(false);
  const [title, setTitle] = createSignal('');
  const [startingTrial, setStartingTrial] = createSignal(false);
  const reportingFeatureId = () => reportingCatalog()?.id ?? '';

  const isLocked = () =>
    licenseLoaded() &&
    reportingFeatureId() !== '' &&
    !hasFeature(reportingFeatureId());
  const canStartTrial = () => entitlements()?.trial_eligible !== false;
  const isReportingEnabled = () =>
    licenseLoaded() &&
    reportingFeatureId() !== '' &&
    hasFeature(reportingFeatureId());
  const upgradeActionUrl = () =>
    reportingFeatureId() === '' ? '' : getUpgradeActionUrlOrFallback(reportingFeatureId());

  onMount(() => {
    loadLicenseStatus();
  });

  createEffect((wasVisible: boolean) => {
    const visible = isLocked();
    if (visible && !wasVisible) {
      trackPaywallViewed(reportingFeatureId(), 'settings_reporting_panel');
    }
    return visible;
  }, false);

  createEffect(() => {
    if (reportingCatalog() || reportingCatalogLoading() || reportingCatalogRequested()) {
      return;
    }

    void (async () => {
      setReportingCatalogRequested(true);
      setReportingCatalogLoading(true);
      setReportingCatalogError('');
      try {
        const request = buildReportingCatalogRequest();
        const response = await apiFetch(request.url);
        if (!response.ok) {
          const text = await response.text();
          throw new Error(text || getReportingCatalogErrorMessage());
        }

        setReportingCatalog(parseReportingCatalog(await response.json()));
      } catch (error) {
        console.error('Reporting catalog error:', error);
        setReportingCatalogError(
          error instanceof Error ? error.message : getReportingCatalogErrorMessage(),
        );
      } finally {
        setReportingCatalogLoading(false);
      }
    })();
  });

  createEffect(() => {
    const performanceReport = reportingCatalog()?.performanceReport;
    if (!performanceReport) {
      return;
    }
    if (
      format() === null ||
      !performanceReport.formats.some((candidate) => candidate.value === format())
    ) {
      setFormat(performanceReport.defaultFormat);
    }
    if (
      range() === null ||
      !performanceReport.ranges.some((candidate) => candidate.key === range())
    ) {
      setRange(performanceReport.defaultRange);
    }
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
      const performanceReport = reportingCatalog()?.performanceReport;
      if (!performanceReport) {
        throw new Error(getReportingGenerateErrorMessage());
      }
      const selectedFormat = format() ?? performanceReport.defaultFormat;
      const selectedRange = range() ?? performanceReport.defaultRange;
      const start = getReportingRangeStart(selectedRange, now, performanceReport);
      const request = buildReportingRequest({
        end: now.toISOString(),
        format: selectedFormat,
        metricType: metricType(),
        now,
        resources,
        start: start.toISOString(),
        title: title(),
      }, performanceReport);

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
      const inventoryDefinition = reportingCatalog()?.vmInventoryExport;
      if (!inventoryDefinition) {
        throw new Error(getReportingInventoryExportErrorMessage());
      }
      const request = buildVMInventoryExportRequest(new Date(), inventoryDefinition);
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
    isLocked,
    isReportingEnabled,
    metricType,
    range,
    reportingCatalog,
    reportingCatalogError,
    reportingCatalogLoading,
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
