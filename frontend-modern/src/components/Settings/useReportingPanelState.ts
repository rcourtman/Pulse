import { createEffect, createSignal, onMount } from 'solid-js';
import { apiErrorFromResponse, apiFetch } from '@/utils/apiClient';
import { showSuccess, showWarning } from '@/utils/toast';
import type { SelectedResource } from '@/components/Settings/ResourcePicker';
import { hasFeature, runtimeCapabilitiesLoaded } from '@/stores/license';
import { getUpgradeActionDestination } from '@/stores/licenseCommercial';
import { presentationPolicyHidesUpgradePrompts } from '@/stores/sessionPresentationPolicy';
import { loadRuntimeCapabilities } from '@/stores/license';
import {
  getReportingCatalogErrorMessage,
  getReportingGenerateErrorMessage,
  getReportingGenerateSelectionRequiredMessage,
  getReportingGenerateSuccessMessage,
  getReportingInventoryExportErrorMessage,
  getReportingInventoryExportSuccessMessage,
  resolveReportingDownloadFilename,
} from '@/utils/reportingPresentation';
import {
  buildReportingRequest,
  getReportingRangeStart,
  type ReportingRangeValue,
} from '@/components/Settings/reportingPanelModel';
import { buildVMInventoryExportRequest } from '@/components/Settings/reportingInventoryExportModel';
import {
  buildLegacyReportingCatalogFallback,
  buildReportingCatalogRequest,
  parseReportingCatalog,
  type ReportingCatalog,
  type ReportingFormat,
} from '@/components/Settings/reportingCatalogModel';
import {
  buildReportSchedulePayload,
  DEFAULT_REPORT_SCHEDULE_FORM,
  normalizeReportSchedule,
  parseReportSchedulesResponse,
  scheduleToForm,
  scheduleToSelectedResources,
  type ReportSchedule,
  type ReportScheduleFormState,
} from '@/components/Settings/reportingSchedulesModel';

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
  const [reportingCatalogAttempted, setReportingCatalogAttempted] = createSignal(false);
  const [title, setTitle] = createSignal('');
  const [reportSchedules, setReportSchedules] = createSignal<ReportSchedule[]>([]);
  const [reportSchedulesLoading, setReportSchedulesLoading] = createSignal(false);
  const [reportSchedulesAttempted, setReportSchedulesAttempted] = createSignal(false);
  const [reportSchedulesError, setReportSchedulesError] = createSignal('');
  const [scheduleFormOpen, setScheduleFormOpen] = createSignal(false);
  const [scheduleForm, setScheduleForm] = createSignal<ReportScheduleFormState>(
    DEFAULT_REPORT_SCHEDULE_FORM(),
  );
  const [scheduleResources, setScheduleResources] = createSignal<SelectedResource[]>([]);
  const [savingSchedule, setSavingSchedule] = createSignal(false);
  const [runningScheduleID, setRunningScheduleID] = createSignal('');
  const [deletingScheduleID, setDeletingScheduleID] = createSignal('');
  const reportingFeatureId = () => reportingCatalog()?.id ?? '';
  const showUpgradePrompts = () => !presentationPolicyHidesUpgradePrompts();

  const isLocked = () =>
    runtimeCapabilitiesLoaded() && reportingFeatureId() !== '' && !hasFeature(reportingFeatureId());
  const isReportingEnabled = () =>
    runtimeCapabilitiesLoaded() && reportingFeatureId() !== '' && hasFeature(reportingFeatureId());
  const upgradeDestination = () =>
    reportingFeatureId() === ''
      ? getUpgradeActionDestination('')
      : getUpgradeActionDestination(reportingFeatureId());

  onMount(() => {
    loadRuntimeCapabilities();
  });

  const loadReportingCatalog = async () => {
    if (reportingCatalogLoading()) {
      return;
    }

    setReportingCatalogAttempted(true);
    setReportingCatalogLoading(true);
    setReportingCatalogError('');
    try {
      const request = buildReportingCatalogRequest();
      const response = await apiFetch(request.url);
      if (!response.ok) {
        if (response.status === 404) {
          setReportingCatalog(buildLegacyReportingCatalogFallback());
          return;
        }
        throw await apiErrorFromResponse(response, getReportingCatalogErrorMessage());
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
  };

  createEffect(() => {
    if (reportingCatalog() || reportingCatalogLoading() || reportingCatalogAttempted()) {
      return;
    }

    void loadReportingCatalog();
  });

  const loadReportSchedules = async () => {
    if (reportSchedulesLoading()) return;
    setReportSchedulesAttempted(true);
    setReportSchedulesLoading(true);
    setReportSchedulesError('');
    try {
      const response = await apiFetch('/api/admin/reports/schedules');
      if (!response.ok) {
        throw await apiErrorFromResponse(response, 'Failed to load report schedules');
      }
      setReportSchedules(parseReportSchedulesResponse(await response.json()));
    } catch (error) {
      console.error('Report schedules error:', error);
      setReportSchedulesError(
        error instanceof Error ? error.message : 'Failed to load report schedules',
      );
    } finally {
      setReportSchedulesLoading(false);
    }
  };

  createEffect(() => {
    if (!isReportingEnabled() || reportSchedulesAttempted() || reportSchedulesLoading()) {
      return;
    }
    void loadReportSchedules();
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

  const downloadResponseBlob = async (filename: string, response: Response) => {
    const blob = await response.blob();
    const url = window.URL.createObjectURL(blob);
    const anchor = document.createElement('a');
    anchor.href = url;
    anchor.download = resolveReportingDownloadFilename(
      response.headers.get('Content-Disposition'),
      filename,
    );
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
      const request = buildReportingRequest(
        {
          end: now.toISOString(),
          format: selectedFormat,
          metricType: metricType(),
          now,
          resources,
          start: start.toISOString(),
          title: title(),
        },
        performanceReport,
      );

      const response = await apiFetch(request.request.url, request.request.init);
      if (!response.ok) {
        throw await apiErrorFromResponse(response, getReportingGenerateErrorMessage());
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
        throw await apiErrorFromResponse(response, getReportingInventoryExportErrorMessage());
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

  const updateScheduleForm = (patch: Partial<ReportScheduleFormState>) => {
    setScheduleForm((current) => ({ ...current, ...patch }));
  };

  const startCreateSchedule = () => {
    setScheduleForm(DEFAULT_REPORT_SCHEDULE_FORM());
    setScheduleResources([]);
    setScheduleFormOpen(true);
  };

  const startEditSchedule = (schedule: ReportSchedule) => {
    const normalized = normalizeReportSchedule(schedule);
    setScheduleForm(scheduleToForm(normalized));
    setScheduleResources(scheduleToSelectedResources(normalized));
    setScheduleFormOpen(true);
  };

  const closeScheduleForm = () => {
    setScheduleFormOpen(false);
    setScheduleForm(DEFAULT_REPORT_SCHEDULE_FORM());
    setScheduleResources([]);
  };

  const saveReportSchedule = async () => {
    if (savingSchedule()) return;
    const form = scheduleForm();
    const payload = buildReportSchedulePayload(form, scheduleResources());
    if (!payload.name) {
      showWarning('Schedule name is required');
      return;
    }
    if ((payload.scope.resources?.length ?? 0) === 0 && (payload.scope.tags?.length ?? 0) === 0) {
      showWarning('Select resources or enter at least one tag');
      return;
    }

    setSavingSchedule(true);
    try {
      const isUpdate = form.id !== '';
      const response = await apiFetch(
        isUpdate
          ? `/api/admin/reports/schedules/${encodeURIComponent(form.id)}`
          : '/api/admin/reports/schedules',
        {
          method: isUpdate ? 'PUT' : 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(payload),
        },
      );
      if (!response.ok) {
        throw await apiErrorFromResponse(response, 'Failed to save report schedule');
      }
      const saved = normalizeReportSchedule(await response.json());
      setReportSchedules((current) => {
        const index = current.findIndex((schedule) => schedule.id === saved.id);
        if (index < 0) return [...current, saved];
        const next = current.slice();
        next[index] = saved;
        return next;
      });
      closeScheduleForm();
      showSuccess('Report schedule saved');
    } catch (error) {
      console.error('Save report schedule error:', error);
      showWarning(error instanceof Error ? error.message : 'Failed to save report schedule');
    } finally {
      setSavingSchedule(false);
    }
  };

  const updateReportSchedule = async (schedule: ReportSchedule) => {
    const normalized = normalizeReportSchedule(schedule);
    const response = await apiFetch(
      `/api/admin/reports/schedules/${encodeURIComponent(normalized.id)}`,
      {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(normalized),
      },
    );
    if (!response.ok) {
      throw await apiErrorFromResponse(response, 'Failed to update report schedule');
    }
    const saved = normalizeReportSchedule(await response.json());
    setReportSchedules((current) =>
      current.map((candidate) => (candidate.id === saved.id ? saved : candidate)),
    );
  };

  const toggleReportSchedule = async (schedule: ReportSchedule) => {
    try {
      await updateReportSchedule({ ...schedule, enabled: !schedule.enabled });
      showSuccess(schedule.enabled ? 'Report schedule disabled' : 'Report schedule enabled');
    } catch (error) {
      console.error('Toggle report schedule error:', error);
      showWarning(error instanceof Error ? error.message : 'Failed to update report schedule');
    }
  };

  const runReportScheduleNow = async (schedule: ReportSchedule) => {
    if (runningScheduleID()) return;
    setRunningScheduleID(schedule.id);
    try {
      const response = await apiFetch(
        `/api/admin/reports/schedules/${encodeURIComponent(schedule.id)}/run`,
        { method: 'POST' },
      );
      if (!response.ok) {
        throw await apiErrorFromResponse(response, 'Failed to run report schedule');
      }
      const body = await response.json();
      if (body?.schedule) {
        const updated = normalizeReportSchedule(body.schedule);
        setReportSchedules((current) =>
          current.map((candidate) => (candidate.id === updated.id ? updated : candidate)),
        );
      } else {
        await loadReportSchedules();
      }
      showSuccess('Report schedule ran');
    } catch (error) {
      console.error('Run report schedule error:', error);
      showWarning(error instanceof Error ? error.message : 'Failed to run report schedule');
      await loadReportSchedules();
    } finally {
      setRunningScheduleID('');
    }
  };

  const deleteReportSchedule = async (schedule: ReportSchedule) => {
    if (deletingScheduleID()) return;
    if (!window.confirm(`Delete ${schedule.name}?`)) return;
    setDeletingScheduleID(schedule.id);
    try {
      const response = await apiFetch(
        `/api/admin/reports/schedules/${encodeURIComponent(schedule.id)}`,
        {
          method: 'DELETE',
        },
      );
      if (!response.ok) {
        throw await apiErrorFromResponse(response, 'Failed to delete report schedule');
      }
      setReportSchedules((current) => current.filter((candidate) => candidate.id !== schedule.id));
      showSuccess('Report schedule deleted');
    } catch (error) {
      console.error('Delete report schedule error:', error);
      showWarning(error instanceof Error ? error.message : 'Failed to delete report schedule');
    } finally {
      setDeletingScheduleID('');
    }
  };

  return {
    closeScheduleForm,
    deleteReportSchedule,
    deletingScheduleID,
    exportingInventory,
    format,
    handleExportVMInventory,
    generating,
    handleGenerate,
    isLocked,
    isReportingEnabled,
    metricType,
    range,
    reportSchedules,
    reportSchedulesError,
    reportSchedulesLoading,
    reportingCatalog,
    reportingCatalogError,
    reportingCatalogLoading,
    reloadReportingCatalog: () => {
      if (reportingCatalogLoading()) {
        return;
      }
      void loadReportingCatalog();
    },
    reloadReportSchedules: () => {
      if (reportSchedulesLoading()) return;
      void loadReportSchedules();
    },
    runReportScheduleNow,
    runningScheduleID,
    saveReportSchedule,
    savingSchedule,
    scheduleForm,
    scheduleFormOpen,
    scheduleResources,
    selectedResources,
    setFormat,
    setMetricType,
    setRange,
    setScheduleResources,
    setSelectedResources,
    setTitle,
    showUpgradePrompts,
    startCreateSchedule,
    startEditSchedule,
    title,
    toggleReportSchedule,
    updateScheduleForm,
    upgradeDestination,
  };
};
