import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

/* ------------------------------------------------------------------ */
/*  Mocks                                                              */
/* ------------------------------------------------------------------ */

const mockInitialDataReceived = vi.hoisted(() => vi.fn<() => boolean>(() => true));
const capabilityState = vi.hoisted(() => ({ businessEstate: true }));
const policyState = vi.hoisted(() => ({ hidesUpgradePrompts: false }));
const mockNavigate = vi.hoisted(() => vi.fn());

vi.mock('@/contexts/appRuntime', () => ({
  useWebSocket: () => ({
    initialDataReceived: mockInitialDataReceived,
  }),
}));

vi.mock('@/stores/sessionCapabilities', () => ({
  sessionCapabilities: () => ({ demoMode: false, businessEstate: capabilityState.businessEstate }),
}));

vi.mock('@/stores/sessionPresentationPolicy', () => ({
  presentationPolicyHidesUpgradePrompts: () => policyState.hidesUpgradePrompts,
}));

vi.mock('@solidjs/router', () => ({
  useNavigate: () => mockNavigate,
}));

/* ------------------------------------------------------------------ */
/*  Helpers                                                            */
/* ------------------------------------------------------------------ */

const DISMISSED_KEY = 'pulse-business-estate-dismissed';
const FIRST_SEEN_KEY = 'pulse-business-estate-first-seen';
const STAR_DISMISSED_KEY = 'pulse-github-star-dismissed';
const STAR_SNOOZED_KEY = 'pulse-github-star-snoozed-until';

const YESTERDAY = '2020-01-01';

async function renderCard() {
  const mod = await import('../BusinessEstateCard');
  render(() => <mod.BusinessEstateCard />);
}

/** Seed the state under which the card is expected to show. */
function seedEligibleState() {
  capabilityState.businessEstate = true;
  policyState.hidesUpgradePrompts = false;
  localStorage.setItem(STAR_DISMISSED_KEY, 'true');
  localStorage.setItem(FIRST_SEEN_KEY, YESTERDAY);
}

function queryCard() {
  return screen.queryByText('Monitoring a business environment?');
}

/* ------------------------------------------------------------------ */
/*  Tests                                                              */
/* ------------------------------------------------------------------ */

describe('BusinessEstateCard', () => {
  beforeEach(() => {
    localStorage.clear();
    mockNavigate.mockReset();
    mockInitialDataReceived.mockReturnValue(true);
    capabilityState.businessEstate = true;
    policyState.hidesUpgradePrompts = false;
  });

  afterEach(() => {
    cleanup();
    localStorage.clear();
  });

  it('shows for an eligible business estate on a later day', async () => {
    seedEligibleState();

    await renderCard();

    await waitFor(() => expect(queryCard()).not.toBeNull());
    expect(screen.getByRole('button', { name: 'See business plans' })).toBeTruthy();
    expect(screen.getByRole('button', { name: 'This is a homelab' })).toBeTruthy();
  });

  it('stays hidden when the session is not a business estate', async () => {
    seedEligibleState();
    capabilityState.businessEstate = false;

    await renderCard();

    expect(queryCard()).toBeNull();
  });

  it('stays hidden when the presentation policy hides upgrade prompts', async () => {
    seedEligibleState();
    policyState.hidesUpgradePrompts = true;

    await renderCard();

    expect(queryCard()).toBeNull();
  });

  it('stays hidden until initial data is received', async () => {
    seedEligibleState();
    mockInitialDataReceived.mockReturnValue(false);

    await renderCard();

    expect(queryCard()).toBeNull();
  });

  it('waits until the GitHub star prompt has been interacted with', async () => {
    seedEligibleState();
    localStorage.removeItem(STAR_DISMISSED_KEY);
    localStorage.removeItem(STAR_SNOOZED_KEY);

    await renderCard();

    expect(queryCard()).toBeNull();
  });

  it('treats a snoozed star prompt as interacted', async () => {
    seedEligibleState();
    localStorage.removeItem(STAR_DISMISSED_KEY);
    localStorage.setItem(STAR_SNOOZED_KEY, '2099-01-01');

    await renderCard();

    await waitFor(() => expect(queryCard()).not.toBeNull());
  });

  it('records the first qualifying day and stays quiet that day', async () => {
    seedEligibleState();
    localStorage.removeItem(FIRST_SEEN_KEY);

    await renderCard();

    expect(queryCard()).toBeNull();
    await waitFor(() => expect(localStorage.getItem(FIRST_SEEN_KEY)).not.toBeNull());
    const recorded = (localStorage.getItem(FIRST_SEEN_KEY) ?? '').replace(/"/g, '');
    expect(recorded).toBe(new Date().toISOString().split('T')[0]);
  });

  it('never shows again after the close control', async () => {
    seedEligibleState();

    await renderCard();
    await waitFor(() => expect(queryCard()).not.toBeNull());

    fireEvent.click(screen.getByRole('button', { name: "Close and don't show again" }));

    expect(queryCard()).toBeNull();
    expect(JSON.parse(localStorage.getItem(DISMISSED_KEY) ?? 'false')).toBe(true);
  });

  it('never shows again after "This is a homelab"', async () => {
    seedEligibleState();

    await renderCard();
    await waitFor(() => expect(queryCard()).not.toBeNull());

    fireEvent.click(screen.getByRole('button', { name: 'This is a homelab' }));

    expect(queryCard()).toBeNull();
    expect(JSON.parse(localStorage.getItem(DISMISSED_KEY) ?? 'false')).toBe(true);
  });

  it('navigates to plan selection and dismisses permanently on the primary action', async () => {
    seedEligibleState();

    await renderCard();
    await waitFor(() => expect(queryCard()).not.toBeNull());

    fireEvent.click(screen.getByRole('button', { name: 'See business plans' }));

    expect(mockNavigate).toHaveBeenCalledWith(
      '/settings/pulse-intelligence/billing/plan?intent=self_hosted_plan&source=estate-card',
    );
    expect(queryCard()).toBeNull();
    expect(JSON.parse(localStorage.getItem(DISMISSED_KEY) ?? 'false')).toBe(true);
  });

  it('dismisses permanently on Escape', async () => {
    seedEligibleState();

    await renderCard();
    await waitFor(() => expect(queryCard()).not.toBeNull());

    fireEvent.keyDown(document, { key: 'Escape' });

    await waitFor(() => expect(queryCard()).toBeNull());
    expect(JSON.parse(localStorage.getItem(DISMISSED_KEY) ?? 'false')).toBe(true);
  });
});
