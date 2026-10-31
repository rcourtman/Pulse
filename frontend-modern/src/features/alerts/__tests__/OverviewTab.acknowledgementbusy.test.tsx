import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { DEFAULT_LOCALE, setActiveLocale } from '@/i18n';
import { createAlertAcknowledgementRequests } from '@/stores/alertAcknowledgementRequests';
import type { Alert, AlertDeliveryDiagnosis } from '@/types/api';
import {
  getAlertOverviewBulkAcknowledgeLabel,
  getAlertOverviewPrimaryActionLabel,
} from '@/utils/alertOverviewPresentation';

vi.mock('@solidjs/router', () => ({
  useLocation: () => ({ hash: '', pathname: '/alerts', search: '', query: {} }),
  A: (props: Record<string, unknown>) => props.children,
}));

type BulkAcknowledgeResult = {
  results: Array<{ alertIdentifier: string; success: boolean; error?: string }>;
};

const getDeliveryDiagnoses = vi.fn<() => Promise<AlertDeliveryDiagnosis[]>>();
const bulkAcknowledge = vi.fn<(alertIdentifiers: string[]) => Promise<BulkAcknowledgeResult>>();

vi.mock('@/api/alerts', () => ({
  AlertsAPI: {
    get getDeliveryDiagnoses() {
      return getDeliveryDiagnoses;
    },
    get bulkAcknowledge() {
      return bulkAcknowledge;
    },
  },
}));

vi.mock('@/stores/notifications', () => ({
  notificationStore: { success: vi.fn(), error: vi.fn() },
}));

vi.mock('@/utils/logger', () => ({
  logger: { error: vi.fn() },
}));

vi.mock('@/components/Alerts/InvestigateAlertButton', () => ({
  InvestigateAlertButton: () => null,
}));

import { OverviewTab } from '../OverviewTab';

function makeAlert(id: string): Alert {
  return {
    id,
    resourceId: `vm-${id}`,
    resourceName: `VM ${id}`,
    type: 'cpu',
    level: 'warning',
    message: `High CPU on VM ${id}`,
    startTime: new Date().toISOString(),
    acknowledged: false,
    node: 'node1',
  } as Alert;
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((settle) => {
    resolve = settle;
  });
  return { promise, resolve };
}

function defaultProps(overrides: Record<string, unknown> = {}) {
  return {
    overrides: [] as never[],
    activeAlerts: {} as Record<string, Alert>,
    updateAlert: vi.fn(),
    acknowledgementRequests: createAlertAcknowledgementRequests(),
    showQuickTip: () => false,
    dismissQuickTip: vi.fn(),
    showAcknowledged: () => true,
    setShowAcknowledged: vi.fn(),
    alertsDisabled: () => false,
    ...overrides,
  };
}

const acknowledgeLabel = () =>
  getAlertOverviewPrimaryActionLabel({ acknowledged: false, processing: false });
const processingLabel = () =>
  getAlertOverviewPrimaryActionLabel({ acknowledged: false, processing: true });

describe('OverviewTab cards during a global acknowledgement', () => {
  beforeEach(() => {
    setActiveLocale(DEFAULT_LOCALE);
    getDeliveryDiagnoses.mockReset();
    getDeliveryDiagnoses.mockResolvedValue([]);
    bulkAcknowledge.mockReset();
  });

  afterEach(() => {
    cleanup();
    setActiveLocale(DEFAULT_LOCALE);
  });

  it('shows the cards it carries as processing until it settles', async () => {
    const response = deferred<BulkAcknowledgeResult>();
    bulkAcknowledge.mockReturnValueOnce(response.promise);
    const activeAlerts = { a1: makeAlert('a1'), a2: makeAlert('a2') };

    render(() => <OverviewTab {...defaultProps({ activeAlerts })} />);
    expect(screen.getAllByRole('button', { name: acknowledgeLabel() })).toHaveLength(2);

    fireEvent.click(
      screen.getByRole('button', { name: getAlertOverviewBulkAcknowledgeLabel(2, false) }),
    );
    expect(bulkAcknowledge).toHaveBeenCalledWith(['a1', 'a2']);
    const busyCards = screen.getAllByRole('button', { name: processingLabel() });
    expect(busyCards).toHaveLength(2);
    busyCards.forEach((button) => expect((button as HTMLButtonElement).disabled).toBe(true));
    expect(
      screen.getByRole('button', { name: getAlertOverviewBulkAcknowledgeLabel(2, true) }),
    ).toBeTruthy();

    // Both fail, so both cards stay; a settled global request holds neither.
    response.resolve({
      results: [
        { alertIdentifier: 'a1', success: false },
        { alertIdentifier: 'a2', success: false },
      ],
    });
    await waitFor(
      () => expect(screen.getAllByRole('button', { name: acknowledgeLabel() })).toHaveLength(2),
      { timeout: 500 },
    );
    expect(screen.queryAllByRole('button', { name: processingLabel() })).toHaveLength(0);
  });

  it('shows a global acknowledgement from before a remount as still in progress', async () => {
    const response = deferred<BulkAcknowledgeResult>();
    bulkAcknowledge.mockReturnValueOnce(response.promise);
    const props = defaultProps({ activeAlerts: { a1: makeAlert('a1') } });

    render(() => <OverviewTab {...props} />);
    fireEvent.click(
      screen.getByRole('button', { name: getAlertOverviewBulkAcknowledgeLabel(1, false) }),
    );
    cleanup();

    // The overview mounts again over the same app-wide request owner.
    render(() => <OverviewTab {...props} />);
    const busyCard = screen.getByRole('button', { name: processingLabel() });
    expect((busyCard as HTMLButtonElement).disabled).toBe(true);
    const globalButton = screen.getByRole('button', {
      name: getAlertOverviewBulkAcknowledgeLabel(1, true),
    }) as HTMLButtonElement;
    expect(globalButton.disabled).toBe(true);

    response.resolve({ results: [{ alertIdentifier: 'a1', success: false }] });
    await waitFor(
      () => expect(screen.getByRole('button', { name: acknowledgeLabel() })).toBeTruthy(),
      { timeout: 500 },
    );
    expect(bulkAcknowledge).toHaveBeenCalledTimes(1);
  });
});
