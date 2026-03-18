import { expect, type Frame, type Locator, type Page } from '@playwright/test';

type StripeCheckoutOptions = {
  email?: string;
  cardholderName?: string;
  cardNumber?: string;
  expiry?: string;
  cvc?: string;
  postalCode?: string;
};

export type StripeCheckoutResult = {
  checkoutSessionID: string | null;
};

const DEFAULT_CARD_NUMBER = '4242424242424242';
const DEFAULT_EXPIRY = '12/34';
const DEFAULT_CVC = '123';
const DEFAULT_POSTAL_CODE = '10001';
const DEFAULT_CARDHOLDER_NAME = 'Pulse E2E';

const STRIPE_URL_PATTERN = /checkout\.stripe\.com/i;

function envOrDefault(name: string, fallback: string): string {
  const raw = process.env[name];
  if (typeof raw !== 'string') {
    return fallback;
  }
  const trimmed = raw.trim();
  return trimmed === '' ? fallback : trimmed;
}

function isPageRaceError(error: unknown): boolean {
  const message = error instanceof Error ? error.message.toLowerCase() : String(error).toLowerCase();
  return (
    message.includes('target page') && message.includes('has been closed')
  ) || message.includes('execution context was destroyed') || message.includes('frame was detached');
}

function hasLeftStripeCheckout(page: Page): boolean {
  if (page.isClosed()) {
    return true;
  }
  let url = '';
  try {
    url = page.url();
  } catch (error) {
    if (isPageRaceError(error)) {
      return true;
    }
    throw error;
  }
  if (url === '') {
    return false;
  }
  return !STRIPE_URL_PATTERN.test(url);
}

async function fillIfVisible(locator: Locator, value: string): Promise<boolean> {
  try {
    if ((await locator.count()) === 0) {
      return false;
    }
    const field = locator.first();
    const visible = await field.isVisible({ timeout: 500 }).catch(() => false);
    if (!visible) {
      return false;
    }
    await field.fill(value);
    return true;
  } catch (error) {
    if (isPageRaceError(error)) {
      return false;
    }
    throw error;
  }
}

async function fillCandidatesOnFrame(frame: Frame, selectors: string[], value: string): Promise<boolean> {
  for (const selector of selectors) {
    const filled = await fillIfVisible(frame.locator(selector), value);
    if (filled) {
      return true;
    }
  }
  return false;
}

async function fillCandidates(page: Page, selectors: string[], value: string): Promise<boolean> {
  if (page.isClosed()) {
    return false;
  }
  const mainFrameFilled = await fillCandidatesOnFrame(page.mainFrame(), selectors, value);
  if (mainFrameFilled) {
    return true;
  }
  let frames: Frame[] = [];
  try {
    frames = page.frames();
  } catch (error) {
    if (isPageRaceError(error)) {
      return false;
    }
    throw error;
  }
  for (const frame of frames) {
    if (frame === page.mainFrame()) {
      continue;
    }
    const filled = await fillCandidatesOnFrame(frame, selectors, value);
    if (filled) {
      return true;
    }
  }
  return false;
}

async function fillWithRetry(
  page: Page,
  selectors: string[],
  value: string,
  timeoutMs: number,
): Promise<boolean> {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    if (page.isClosed()) {
      return false;
    }
    const filled = await fillCandidates(page, selectors, value);
    if (filled) {
      return true;
    }
    await page.waitForTimeout(500);
  }
  return false;
}

async function clickFirstVisibleButton(page: Page, pattern: RegExp): Promise<boolean> {
  if (page.isClosed()) {
    return false;
  }
  const mainFrameButtons = page.mainFrame().getByRole('button', { name: pattern });
  const mainVisible = await mainFrameButtons.first().isVisible({ timeout: 500 }).catch(() => false);
  if (mainVisible) {
    await mainFrameButtons.first().click();
    return true;
  }

  const frames = page.frames();
  for (const frame of frames) {
    if (frame === page.mainFrame()) {
      continue;
    }
    const buttons = frame.getByRole('button', { name: pattern });
    const visible = await buttons.first().isVisible({ timeout: 500 }).catch(() => false);
    if (visible) {
      await buttons.first().click();
      return true;
    }
  }
  return false;
}

export async function completeStripeSandboxCheckout(
  page: Page,
  options: StripeCheckoutOptions = {},
): Promise<StripeCheckoutResult> {
  const checkoutData = {
    email: options.email?.trim() || '',
    cardholderName: options.cardholderName?.trim() || envOrDefault('PULSE_E2E_STRIPE_CARDHOLDER', DEFAULT_CARDHOLDER_NAME),
    cardNumber: options.cardNumber?.trim() || envOrDefault('PULSE_E2E_STRIPE_CARD_NUMBER', DEFAULT_CARD_NUMBER),
    expiry: options.expiry?.trim() || envOrDefault('PULSE_E2E_STRIPE_EXPIRY', DEFAULT_EXPIRY),
    cvc: options.cvc?.trim() || envOrDefault('PULSE_E2E_STRIPE_CVC', DEFAULT_CVC),
    postalCode: options.postalCode?.trim() || envOrDefault('PULSE_E2E_STRIPE_POSTAL_CODE', DEFAULT_POSTAL_CODE),
  };

  await expect(page).toHaveURL(STRIPE_URL_PATTERN, { timeout: 90_000 });
  const initialStripeURL = page.url();
  const checkoutSessionID = initialStripeURL.match(/\/(cs_[^/?#]+)/)?.[1] ?? null;

  if (checkoutData.email !== '') {
    await fillWithRetry(page, ['input[type="email"]', 'input[name="email"]', 'input#email'], checkoutData.email, 10_000);
    if (hasLeftStripeCheckout(page)) {
      return { checkoutSessionID };
    }
  }

  const cardFilled = await fillWithRetry(page, [
    'input[autocomplete="cc-number"]',
    'input[name="cardNumber"]',
    'input[id*="cardNumber"]',
    'input[aria-label*="Card number"]',
  ], checkoutData.cardNumber, 45_000);
  if (!cardFilled) {
    if (hasLeftStripeCheckout(page)) {
      return { checkoutSessionID };
    }
    throw new Error('Unable to find Stripe card number input');
  }

  const expiryFilled = await fillWithRetry(page, [
    'input[autocomplete="cc-exp"]',
    'input[name="cardExpiry"]',
    'input[id*="cardExpiry"]',
    'input[aria-label*="Expiration"]',
  ], checkoutData.expiry, 20_000);
  if (!expiryFilled) {
    if (hasLeftStripeCheckout(page)) {
      return { checkoutSessionID };
    }
    throw new Error('Unable to find Stripe expiry input');
  }

  const cvcFilled = await fillWithRetry(page, [
    'input[autocomplete="cc-csc"]',
    'input[name="cardCvc"]',
    'input[id*="cardCvc"]',
    'input[aria-label*="Security code"]',
    'input[aria-label*="CVC"]',
  ], checkoutData.cvc, 20_000);
  if (!cvcFilled) {
    if (hasLeftStripeCheckout(page)) {
      return { checkoutSessionID };
    }
    throw new Error('Unable to find Stripe CVC input');
  }

  const cardholderFilled = await fillWithRetry(page, [
    'input[autocomplete="cc-name"]',
    'input[name="billingName"]',
    'input[name="cardholderName"]',
    'input[id*="cardholderName"]',
    'input[aria-label*="Cardholder name"]',
  ], checkoutData.cardholderName, 20_000);
  if (!cardholderFilled) {
    if (hasLeftStripeCheckout(page)) {
      return { checkoutSessionID };
    }
    throw new Error('Unable to find Stripe cardholder input');
  }

  await fillWithRetry(page, [
    'input[autocomplete="postal-code"]',
    'input[autocomplete="billing postal-code"]',
    'input[name="billingPostalCode"]',
    'input[name="billingAddressPostalCode"]',
    'input[id*="billingAddressPostalCode"]',
    'input[aria-label*="Postal code"]',
    'input[aria-label*="ZIP"]',
  ], checkoutData.postalCode, 10_000);
  if (hasLeftStripeCheckout(page)) {
    return { checkoutSessionID };
  }

  const clickPatterns = [
    /start trial/i,
    /start free trial/i,
    /subscribe/i,
    /pay/i,
    /complete/i,
    /continue/i,
  ];

  let clicked = false;
  for (const pattern of clickPatterns) {
    if (await clickFirstVisibleButton(page, pattern)) {
      clicked = true;
      break;
    }
  }
  if (!clicked) {
    if (hasLeftStripeCheckout(page)) {
      return { checkoutSessionID };
    }
    throw new Error('Unable to find Stripe checkout submit button');
  }

  return { checkoutSessionID };
}
