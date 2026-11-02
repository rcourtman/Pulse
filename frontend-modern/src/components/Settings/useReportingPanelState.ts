import { createEffect, createSignal, onMount, onCleanup, untrack } from 'solid-js';
import { createStore, reconcile, unwrap } from 'solid-js/store';
import { apiErrorFromResponse, apiFetch, getOrgID } from '@/utils/apiClient';
import { aiAvailabilityStore } from '@/stores/aiAvailability';
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
  applyReportScheduleKind,
  buildReportSchedulePayload,
  DEFAULT_REPORT_SCHEDULE_FORM,
  normalizeReportSchedule,
  parseReportSchedulesResponse,
  scheduleToForm,
  scheduleToSelectedResources,
  reportScheduleLastRunLabel,
  reportScheduleNeedsRetry,
  type ReportSchedule,
  type ReportScheduleFormState,
  type ReportScheduleKind,
} from '@/components/Settings/reportingSchedulesModel';

export const useReportingPanelState = (isCurrentReader: () => boolean = () => true) => {
  const org = getOrgID() || 'default';
  let disposed = false;
  const controller = new AbortController();
  const isActive = () => !disposed && isCurrentReader() && (getOrgID() || 'default') === org;
  const assertActive = () => {
    if (!isActive()) throw new Error('Reporting reader changed');
  };
  const fetchForReader = async (...[url, init]: Parameters<typeof apiFetch>) => {
    assertActive();
    const response = await apiFetch(url, {
      ...init,
      expectedOrgID: org,
      retry: false,
      signal: controller.signal,
    });
    assertActive();
    return response;
  };
  const readJSON = async (response: Response) => {
    const body = await response.json();
    assertActive();
    return body;
  };
  const [actionError, setActionError] = createSignal('');
  const [successNotice, setSuccessNotice] = createSignal('');
  const showWarning = (message: string) => {
    if (isActive()) {
      setSuccessNotice('');
      setActionError(message);
    }
  };
  const showSuccess = (message: string) => {
    if (isActive()) {
      setActionError('');
      setSuccessNotice(message);
    }
  };
  const clearFeedback = () => {
    setActionError('');
    setSuccessNotice('');
  };
  onCleanup(() => {
    disposed = true;
    controller.abort();
  });
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
  const [scheduleState, setScheduleState] = createStore<{ schedules: ReportSchedule[] }>({
    schedules: [],
  });
  const reportSchedules = () => scheduleState.schedules;
  const setReportSchedules = (
    update: ReportSchedule[] | ((current: ReportSchedule[]) => ReportSchedule[]),
  ) => {
    const next =
      typeof update === 'function'
        ? untrack(() => update(unwrap(scheduleState.schedules)))
        : update;
    setScheduleState('schedules', reconcile(next, { key: 'id' }));
  };
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
  const [updatingSchedule, setUpdatingSchedule] = createSignal(false);
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
    if (!isActive() || reportingCatalogLoading()) {
      return;
    }

    setReportingCatalogAttempted(true);
    setReportingCatalogLoading(true);
    setReportingCatalogError('');
    try {
      const request = buildReportingCatalogRequest();
      const response = await fetchForReader(request.url);
      if (!response.ok) {
        if (response.status === 404) {
          setReportingCatalog(buildLegacyReportingCatalogFallback());
          return;
        }
        throw await apiErrorFromResponse(response, getReportingCatalogErrorMessage());
      }

      setReportingCatalog(parseReportingCatalog(await readJSON(response)));
    } catch (error) {
      if (!isActive()) return;
      setReportingCatalogError(
        error instanceof Error ? error.message : getReportingCatalogErrorMessage(),
      );
    } finally {
      if (isActive()) setReportingCatalogLoading(false);
    }
  };

  createEffect(() => {
    if (reportingCatalog() || reportingCatalogLoading() || reportingCatalogAttempted()) {
      return;
    }

    void loadReportingCatalog();
  });

  const loadReportSchedules = async (afterMutation = false) => {
    if (
      !afterMutation &&
      (savingSchedule() || updatingSchedule() || runningScheduleID() || deletingScheduleID())
    )
      return;
    if (!isActive() || reportSchedulesLoading()) return;
    setReportSchedulesAttempted(true);
    setReportSchedulesLoading(true);
    setReportSchedulesError('');
    try {
      const response = await fetchForReader('/api/admin/reports/schedules');
      if (!response.ok) {
        throw await apiErrorFromResponse(response, 'Failed to load report schedules');
      }
      const body = await readJSON(response);
      // A malformed response is unknown, never evidence of an empty directory.
      if (
        !body ||
        !('schedules' in body) ||
        (body.schedules !== null &&
          (!Array.isArray(body.schedules) ||
            body.schedules.some(
              (schedule: ReportSchedule) =>
                !schedule ||
                typeof schedule.id !== 'string' ||
                !schedule.id ||
                typeof schedule.name !== 'string',
            ) ||
            new Set(body.schedules.map((schedule: ReportSchedule) => schedule.id)).size !==
              body.schedules.length))
      ) {
        throw new Error('Report schedules are unavailable. Refresh to try again.');
      }
      setReportSchedules(parseReportSchedulesResponse(body));
    } catch (error) {
      if (!isActive()) return;
      setReportSchedulesError(
        error instanceof Error ? error.message : 'Failed to load report schedules',
      );
    } finally {
      if (isActive()) setReportSchedulesLoading(false);
    }
  };

  createEffect(() => {
    if (!isReportingEnabled() || reportSchedulesAttempted() || reportSchedulesLoading()) {
      return;
    }
    void loadReportSchedules();
  });

  createEffect(() => {
    if (!isReportingEnabled()) return;
    const active = reportSchedules().some((schedule) =>
      ['running', 'retrying'].includes(schedule.last_run_status || ''),
    );
    if (!active && !reportSchedules().some((schedule) => schedule.enabled)) return;
    const refreshVisibleSchedules = () => {
      if (!document.hidden) void loadReportSchedules();
    };
    const timer = window.setInterval(refreshVisibleSchedules, active ? 5000 : 30000);
    document.addEventListener('visibilitychange', refreshVisibleSchedules);
    onCleanup(() => {
      window.clearInterval(timer);
      document.removeEventListener('visibilitychange', refreshVisibleSchedules);
    });
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

  const canEditSchedules = () =>
    isActive() &&
    isReportingEnabled() &&
    reportSchedulesAttempted() &&
    !reportSchedulesLoading() &&
    !reportSchedulesError() &&
    !savingSchedule() &&
    !updatingSchedule() &&
    !runningScheduleID() &&
    !deletingScheduleID();
  const isCurrentSchedule = (schedule: ReportSchedule) =>
    reportSchedules().some((current) => unwrap(current) === unwrap(schedule));

  const downloadResponseBlob = async (filename: string, response: Response) => {
    const blob = await response.blob();
    assertActive();
    const url = window.URL.createObjectURL(blob);
    const anchor = document.createElement('a');
    anchor.href = url;
    anchor.download = resolveReportingDownloadFilename(
      response.headers.get('Content-Disposition'),
      filename,
    );
    try {
      document.body.appendChild(anchor);
      anchor.click();
    } finally {
      window.URL.revokeObjectURL(url);
      anchor.remove();
    }
  };

  const handleGenerate = async () => {
    if (!isActive() || !isReportingEnabled() || generating()) return;
    clearFeedback();
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

      const response = await fetchForReader(request.request.url, request.request.init);
      if (!response.ok) {
        throw await apiErrorFromResponse(response, getReportingGenerateErrorMessage());
      }

      await downloadResponseBlob(request.filename, response);

      showSuccess(getReportingGenerateSuccessMessage());
    } catch (error) {
      if (!isActive()) return;
      showWarning(error instanceof Error ? error.message : getReportingGenerateErrorMessage());
    } finally {
      if (isActive()) setGenerating(false);
    }
  };

  const handleExportVMInventory = async () => {
    if (!isActive() || !isReportingEnabled() || exportingInventory()) return;
    clearFeedback();

    setExportingInventory(true);
    try {
      const inventoryDefinition = reportingCatalog()?.vmInventoryExport;
      if (!inventoryDefinition) {
        throw new Error(getReportingInventoryExportErrorMessage());
      }
      const request = buildVMInventoryExportRequest(new Date(), inventoryDefinition);
      const response = await fetchForReader(request.request.url);
      if (!response.ok) {
        throw await apiErrorFromResponse(response, getReportingInventoryExportErrorMessage());
      }

      await downloadResponseBlob(request.filename, response);
      showSuccess(getReportingInventoryExportSuccessMessage());
    } catch (error) {
      if (!isActive()) return;
      showWarning(
        error instanceof Error ? error.message : getReportingInventoryExportErrorMessage(),
      );
    } finally {
      if (isActive()) setExportingInventory(false);
    }
  };

  const updateScheduleForm = (patch: Partial<ReportScheduleFormState>) => {
    setScheduleForm((current) => ({ ...current, ...patch }));
  };

  // The Patrol weekly summary only has content when Patrol can run, which is
  // the same session capability that shows the Patrol navigation: AI switched
  // on with a provider configured.
  const patrolDigestAvailable = () => aiAvailabilityStore.enabled === true;

  const setScheduleKind = (kind: ReportScheduleKind) => {
    setScheduleForm((current) => applyReportScheduleKind(current, kind));
    if (kind === 'backup_health') {
      setScheduleResources((current) =>
        current.filter(
          (resource) => resource.type === 'vm' || resource.type === 'system-container',
        ),
      );
    } else if (kind !== 'resources') {
      setScheduleResources([]);
    }
  };

  const startCreateSchedule = () => {
    if (!canEditSchedules()) return;
    clearFeedback();
    setScheduleForm(DEFAULT_REPORT_SCHEDULE_FORM());
    setScheduleResources([]);
    setScheduleFormOpen(true);
  };

  const startEditSchedule = (schedule: ReportSchedule) => {
    if (!canEditSchedules() || !isCurrentSchedule(schedule)) return;
    clearFeedback();
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
    if (!canEditSchedules() || !scheduleFormOpen()) return;
    clearFeedback();
    const form = scheduleForm();
    const payload = buildReportSchedulePayload(form, scheduleResources());
    if (!payload.name) {
      showWarning('Schedule name is required');
      return;
    }
    if (
      payload.kind === 'resources' &&
      (payload.scope.resources?.length ?? 0) === 0 &&
      (payload.scope.tags?.length ?? 0) === 0
    ) {
      showWarning('Select resources or enter at least one tag');
      return;
    }

    setSavingSchedule(true);
    try {
      const isUpdate = form.id !== '';
      const response = await fetchForReader(
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
      const saved = normalizeReportSchedule(await readJSON(response));
      setReportSchedules((current) => {
        const index = current.findIndex((schedule) => schedule.id === saved.id);
        if (index < 0) return [...current, saved];
        const next = current.slice();
        next[index] = saved;
        return next;
      });
      closeScheduleForm();
      await loadReportSchedules(true);
      showSuccess('Report schedule saved');
    } catch (error) {
      if (!isActive()) return;
      showWarning(error instanceof Error ? error.message : 'Failed to save report schedule');
    } finally {
      if (isActive()) setSavingSchedule(false);
    }
  };

  const updateReportSchedule = async (schedule: ReportSchedule) => {
    const normalized = normalizeReportSchedule(schedule);
    const payload = { ...normalized };
    delete payload.delivery_check;
    const response = await fetchForReader(
      `/api/admin/reports/schedules/${encodeURIComponent(normalized.id)}`,
      {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      },
    );
    if (!response.ok) {
      throw await apiErrorFromResponse(response, 'Failed to update report schedule');
    }
    const saved = normalizeReportSchedule(await readJSON(response));
    setReportSchedules((current) =>
      current.map((candidate) => (candidate.id === saved.id ? saved : candidate)),
    );
    await loadReportSchedules(true);
  };

  const toggleReportSchedule = async (schedule: ReportSchedule) => {
    if (!canEditSchedules() || !isCurrentSchedule(schedule)) return;
    clearFeedback();
    const enabled = !schedule.enabled;
    setUpdatingSchedule(true);
    try {
      await updateReportSchedule({ ...schedule, enabled });
      showSuccess(enabled ? 'Report schedule enabled' : 'Report schedule disabled');
    } catch (error) {
      if (!isActive()) return;
      showWarning(error instanceof Error ? error.message : 'Failed to update report schedule');
    } finally {
      if (isActive()) setUpdatingSchedule(false);
    }
  };

  const runReportScheduleNow = async (schedule: ReportSchedule) => {
    if (!canEditSchedules() || !isCurrentSchedule(schedule) || runningScheduleID()) return;
    clearFeedback();
    if (
      schedule.last_run_status === 'unconfirmed' &&
      !window.confirm(
        'The previous email may have been accepted. Check the recipient first. Retry anyway, with a risk of a duplicate?',
      )
    )
      return;
    if (!canEditSchedules() || !isCurrentSchedule(schedule)) return;
    const retry = reportScheduleNeedsRetry(schedule);
    setRunningScheduleID(schedule.id);
    try {
      const response = await fetchForReader(
        `/api/admin/reports/schedules/${encodeURIComponent(schedule.id)}/run${retry ? '?retry=true' : ''}`,
        { method: 'POST' },
      );
      const body = await response
        .clone()
        .json()
        .catch(() => null);
      assertActive();
      if (body?.schedule) {
        const updated = normalizeReportSchedule(body.schedule);
        setReportSchedules((current) =>
          current.map((candidate) => (candidate.id === updated.id ? updated : candidate)),
        );
      }
      await loadReportSchedules(true);
      if (!response.ok) {
        if (body?.schedule) {
          showWarning(body.schedule.last_error || reportScheduleLastRunLabel(body.schedule));
          return;
        }
        throw await apiErrorFromResponse(response, 'Failed to run report schedule');
      }
      if (
        body?.schedule?.last_run_status === 'retrying' ||
        body?.schedule?.last_run_status === 'unconfirmed'
      )
        showWarning(body.schedule.last_error || reportScheduleLastRunLabel(body.schedule));
      else
        showSuccess(
          body?.schedule ? reportScheduleLastRunLabel(body.schedule) : 'Report schedule ran',
        );
    } catch (error) {
      if (!isActive()) return;
      showWarning(error instanceof Error ? error.message : 'Failed to run report schedule');
      await loadReportSchedules(true);
    } finally {
      if (isActive()) setRunningScheduleID('');
    }
  };

  const deleteReportSchedule = async (schedule: ReportSchedule) => {
    if (!canEditSchedules() || !isCurrentSchedule(schedule) || deletingScheduleID()) return;
    clearFeedback();
    if (!window.confirm(`Delete ${schedule.name}?`)) return;
    if (!canEditSchedules() || !isCurrentSchedule(schedule)) return;
    setDeletingScheduleID(schedule.id);
    try {
      const response = await fetchForReader(
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
      if (!isActive()) return;
      showWarning(error instanceof Error ? error.message : 'Failed to delete report schedule');
    } finally {
      if (isActive()) setDeletingScheduleID('');
    }
  };

  return {
    actionError,
    successNotice,
    canEditSchedules,
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
    patrolDigestAvailable,
    range,
    reportSchedules,
    reportSchedulesError,
    reportSchedulesLoading,
    reportingCatalog,
    reportingCatalogError,
    reportingCatalogLoading,
    reloadReportingCatalog: () => {
      if (!isActive() || reportingCatalogLoading()) {
        return;
      }
      void loadReportingCatalog();
    },
    reloadReportSchedules: () => {
      if (!isActive() || reportSchedulesLoading()) return;
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
    setScheduleKind,
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
