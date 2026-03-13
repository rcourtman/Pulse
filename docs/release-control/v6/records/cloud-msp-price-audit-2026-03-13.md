# Cloud/MSP Stripe Price Audit

- Date: `2026-03-13`
- Decisions:
  - `cloud-msp-price-id-propagation`
  - `cloud-msp-stripe-prices`
- Scope:
  - `pulse-pro/OPERATIONS.md`
  - `pulse-pro/V6_LAUNCH_CHECKLIST.md`
  - `pulse-pro/license-server/secrets.env.template`
  - Live Stripe account configured by local `secrets/stripe/secret_key`

## Verification Method

1. Extracted the canonical Cloud/MSP v6 price IDs from
   `pulse-pro/license-server/secrets.env.template`.
2. Confirmed every canonical ID also appears in:
   - `pulse-pro/OPERATIONS.md`
   - `pulse-pro/V6_LAUNCH_CHECKLIST.md`
3. Queried Stripe for each price with `stripe prices retrieve <price_id>` using
   the configured live secret key.

## Canonical Cloud/MSP Price IDs

### Cloud

- `price_1T5kflBrHBocJIGHUqPv1dzV` `cloud_starter` monthly `$29`
- `price_1T5kfmBrHBocJIGHTS3ymKxM` `cloud_starter` annual `$249`
- `price_1T5kfnBrHBocJIGHATQJr79D` `cloud_founding` monthly `$19`
- `price_1T5kg2BrHBocJIGHmkoF0zXY` `cloud_power` monthly `$49`
- `price_1T5kg3BrHBocJIGH2EtzKofV` `cloud_power` annual `$449`
- `price_1T5kg4BrHBocJIGHHa8Ecqho` `cloud_max` monthly `$79`
- `price_1T5kg5BrHBocJIGH5AIJ4nVc` `cloud_max` annual `$699`

### MSP

- `price_1T5kgTBrHBocJIGHjOs15LI2` `msp_starter` monthly `$149`
- `price_1T5kgUBrHBocJIGHT6PiOn6x` `msp_starter` annual `$1,490`
- `price_1T5kgVBrHBocJIGHulNsCTb1` `msp_growth` monthly `$249`
- `price_1T5kgWBrHBocJIGHTuaNjnJ2` `msp_growth` annual `$2,490`
- `price_1T5kgWBrHBocJIGHo40iFeRd` `msp_scale` monthly `$399`
- `price_1T5kgXBrHBocJIGHWlOgTyGV` `msp_scale` annual `$3,990`

## Result

- All 13 canonical Cloud/MSP price IDs are present in the three governed
  `pulse-pro` operational surfaces above.
- Stripe returned a live recurring price object for all 13 IDs.
- Every verified price was `active=true` and `livemode=true`.
- The Stripe amounts and billing intervals matched the governed Cloud/MSP
  pricing contract recorded in `pulse-pro`.

## Outcome

The two release-ready decisions are no longer open work:

1. Cloud/MSP price IDs have been propagated into the required operational docs,
   launch checklist, and runtime env mapping template.
2. The governed Cloud/MSP price IDs already exist as active live Stripe prices.
