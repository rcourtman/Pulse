import { describe, expect, it } from 'vitest';

import {
  MAIL_GATEWAY_PHONE_COLUMNS,
  MAIL_GATEWAY_PHONE_COLUMN_WIDTHS,
} from '../ProxmoxMailGatewayTable';

describe('ProxmoxMailGatewayTable phone presentation', () => {
  it('keeps fleet context and mail-flow counters visible without scrolling', () => {
    expect(MAIL_GATEWAY_PHONE_COLUMNS).toEqual([
      'instance',
      'nodes',
      'uptime',
      'mail',
      'queue',
      'deferred',
    ]);
    expect(MAIL_GATEWAY_PHONE_COLUMN_WIDTHS).toEqual({
      instance: 30,
      nodes: 12,
      uptime: 16,
      mail: 14,
      queue: 14,
      deferred: 14,
    });
  });
});
