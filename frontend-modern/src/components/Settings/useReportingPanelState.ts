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
} from '@/utils/reportingPresentation';
import { runStartProTrialAction } from '@/utils/trialStartAction';
import {
  buildReportingRequest,
  getReportingRangeStart,
  type ReportingFormat,
  type ReportingRangeValue,
} from '@/components/Settings/reportingPanelModel';

export const useReportingPanelState = () => {
  const [selectedResources, setSelectedResources] = createSignal<SelectedResource[]>([]);
  const [metricType, setMetricType] = createSignal('');
  const [format, setFormat] = createSignal<ReportingFormat>('pdf');
  const [range, setRange] = createSignal<ReportingRangeValue>('24h');
  const [generating, setGenerating] = createSignal(false);
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

      const blob = await response.blob();
      const url = window.URL.createObjectURL(blob);
      const anchor = document.createElement('a');
      anchor.href = url;
      anchor.download = request.filename;
      document.body.appendChild(anchor);
      anchor.click();
      window.URL.revokeObjectURL(url);
      document.body.removeChild(anchor);

      showSuccess(getReportingGenerateSuccessMessage());
    } catch (error) {
      console.error('Report generation error:', error);
      showWarning(error instanceof Error ? error.message : getReportingGenerateErrorMessage());
    } finally {
      setGenerating(false);
    }
  };

  return {
    canStartTrial,
    format,
    generating,
    handleGenerate,
    handleStartTrial,
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
