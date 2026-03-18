#!/usr/bin/env node

const usage = `Usage:
  node tests/integration/scripts/create-commercial-cancellation-fixtures.mjs [options]

Options:
  --stripe-key <key>             Stripe secret key (or set STRIPE_API_KEY)
  --monthly-price-id <id>        Legacy monthly price ID
  --annual-price-id <id>         Legacy annual price ID
  --email <email>                Reuse a specific customer email
  --format <json|shell>          Output format (default: json)
  --help                         Show this help
`;

function parseArgs(argv) {
  const args = {
    stripeKey: process.env.STRIPE_API_KEY || '',
    monthlyPriceID: process.env.PULSE_CCR_MONTHLY_LEGACY_PRICE_ID || '',
    annualPriceID: process.env.PULSE_CCR_ANNUAL_LEGACY_PRICE_ID || '',
    email: '',
    format: 'json',
  };
  for (let i = 0; i < argv.length; i += 1) {
    const arg = argv[i];
    if (arg === '--help') {
      console.log(usage);
      process.exit(0);
    }
    if (arg === '--stripe-key') {
      args.stripeKey = argv[++i] || '';
      continue;
    }
    if (arg === '--monthly-price-id') {
      args.monthlyPriceID = argv[++i] || '';
      continue;
    }
    if (arg === '--annual-price-id') {
      args.annualPriceID = argv[++i] || '';
      continue;
    }
    if (arg === '--email') {
      args.email = argv[++i] || '';
      continue;
    }
    if (arg === '--format') {
      args.format = argv[++i] || '';
      continue;
    }
    throw new Error(`Unknown argument: ${arg}`);
  }
  if (!args.stripeKey) {
    throw new Error('Missing Stripe key. Use --stripe-key or STRIPE_API_KEY.');
  }
  if (!args.monthlyPriceID || !args.annualPriceID) {
    throw new Error(
      'Missing legacy price IDs. Use --monthly-price-id/--annual-price-id or PULSE_CCR_*_LEGACY_PRICE_ID.',
    );
  }
  if (!['json', 'shell'].includes(args.format)) {
    throw new Error(`Unsupported format: ${args.format}`);
  }
  return args;
}

async function stripePost(stripeKey, path, form) {
  const response = await fetch(`https://api.stripe.com/v1${path}`, {
    method: 'POST',
    headers: {
      Authorization: `Bearer ${stripeKey}`,
      'Content-Type': 'application/x-www-form-urlencoded',
      Accept: 'application/json',
    },
    body: new URLSearchParams(form),
  });
  const payload = await response.json().catch(() => ({}));
  if (!response.ok) {
    throw new Error(
      `Stripe ${path} failed: ${payload?.error?.message || response.status}`,
    );
  }
  return payload;
}

async function createFixture(stripeKey, { email, priceID, planVersion, suffix }) {
  const clock = await stripePost(stripeKey, '/test_helpers/test_clocks', {
    frozen_time: String(Math.floor(Date.now() / 1000) - 60),
    name: `${planVersion}-${suffix}`,
  });
  const customer = await stripePost(stripeKey, '/customers', {
    email,
    name: `CCR ${planVersion} ${suffix}`,
    test_clock: clock.id,
    source: 'tok_visa',
  });
  const subscription = await stripePost(stripeKey, '/subscriptions', {
    customer: customer.id,
    'items[0][price]': priceID,
    'metadata[plan_version]': planVersion,
  });
  return {
    customer_id: customer.id,
    subscription_id: subscription.id,
    price_id: priceID,
    test_clock_id: clock.id,
    status: subscription.status,
  };
}

function renderShell(result) {
  const lines = [
    `export PULSE_CCR_RETURNER_EMAIL='${result.email}'`,
    `export PULSE_CCR_MONTHLY_CUSTOMER_ID='${result.monthly.customer_id}'`,
    `export PULSE_CCR_MONTHLY_SUBSCRIPTION_ID='${result.monthly.subscription_id}'`,
    `export PULSE_CCR_MONTHLY_LEGACY_PRICE_ID='${result.monthly.price_id}'`,
    `export PULSE_CCR_MONTHLY_TEST_CLOCK_ID='${result.monthly.test_clock_id}'`,
    `export PULSE_CCR_ANNUAL_CUSTOMER_ID='${result.annual.customer_id}'`,
    `export PULSE_CCR_ANNUAL_SUBSCRIPTION_ID='${result.annual.subscription_id}'`,
    `export PULSE_CCR_ANNUAL_LEGACY_PRICE_ID='${result.annual.price_id}'`,
    `export PULSE_CCR_ANNUAL_TEST_CLOCK_ID='${result.annual.test_clock_id}'`,
  ];
  return `${lines.join('\n')}\n`;
}

async function main() {
  const args = parseArgs(process.argv.slice(2));
  const suffix = `${Date.now()}`;
  const email = args.email || `ccr-${suffix}@example.com`;
  const result = {
    email,
    monthly: await createFixture(args.stripeKey, {
      email,
      priceID: args.monthlyPriceID,
      planVersion: 'v5_pro_monthly_grandfathered',
      suffix,
    }),
    annual: await createFixture(args.stripeKey, {
      email,
      priceID: args.annualPriceID,
      planVersion: 'v5_pro_annual_grandfathered',
      suffix,
    }),
  };
  if (args.format === 'shell') {
    process.stdout.write(renderShell(result));
    return;
  }
  process.stdout.write(`${JSON.stringify(result, null, 2)}\n`);
}

main().catch((error) => {
  console.error(error.message || String(error));
  process.exit(1);
});
