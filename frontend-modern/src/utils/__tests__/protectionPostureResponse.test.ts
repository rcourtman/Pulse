import { describe, expect, it } from 'vitest';
import { protectionPolicy, protectionPosture } from '../__fixtures__/protectionPosture';
import {
  parseProtectionPostureBatches,
  ProtectionPostureFormatError,
} from '../protectionPostureResponse';

const response = (row: unknown = protectionPosture()) => ({
  data: [row],
  policy: { ...protectionPolicy },
});

describe('protection posture response boundary', () => {
  it('keeps server-owned assessments, row order, scopes and extra fields without re-evaluation', () => {
    const row = protectionPosture();
    row.state = 'attention';
    row.providerStates[0].permissions = 'partial';
    row.providerStates[0].historyCompleteness = 'partial';
    const result = parseProtectionPostureBatches([
      { ...response(row), meta: { total: 1 }, futureField: true },
      response({ ...protectionPosture('vm:2'), futureField: { harmless: true } }),
    ]);
    expect(result.data).toEqual([
      row,
      { ...protectionPosture('vm:2'), futureField: { harmless: true } },
    ]);
    expect(result.policy).toEqual(protectionPolicy);
    expect(Object.keys(result).sort()).toEqual(['data', 'policy']);
  });

  it.each([null, []])('accepts an explicit empty Go collection %j', (data) => {
    expect(parseProtectionPostureBatches([{ data, policy: protectionPolicy }])).toEqual({
      data: [],
      policy: protectionPolicy,
    });
  });

  it('normalises nil evidence lists without inventing provider or recovery evidence', () => {
    const row = { ...protectionPosture(), evidenceIds: null, repositoryResourceIds: null };
    const result = parseProtectionPostureBatches([
      response({ ...row, providerStates: [{ ...row.providerStates[0], evidenceIds: null }] }),
    ]);
    expect(result.data[0]).toEqual(protectionPosture());
    expect(
      parseProtectionPostureBatches([response({ ...row, providerStates: null })]).data[0]
        .providerStates,
    ).toEqual([]);
  });

  it('permits optional absent/null dates and verification expectation, including unknown provider evidence', () => {
    const row = protectionPosture();
    row.lastAttemptAt = null;
    row.lastVerifiedAt = '2026-10-26T11:00:00Z';
    row.providerStates[0].verificationExpected = false;
    row.providerStates[0].lastSuccessAt = null;
    row.providerStates[0].provider = 'future-provider';
    row.providerStates[0].jobState = 'unknown';
    expect(parseProtectionPostureBatches([response(row)]).data).toEqual([row]);
  });

  it.each([
    ['no batches', []],
    ['null envelope', [null]],
    ['array envelope', [[]]],
    ['no collection', [{ policy: protectionPolicy }]],
    ['object collection', [{ data: {}, policy: protectionPolicy }]],
    ['no policy', [{ data: [] }]],
    ['null row', [response(null)]],
    ['array row', [response([])]],
  ])('rejects %s', (_name, payloads) => {
    expect(() => parseProtectionPostureBatches(payloads)).toThrow(ProtectionPostureFormatError);
  });

  it.each([
    ['subjectResourceId', ''],
    ['subjectResourceId', 112],
    ['state', 'healthy'],
    ['state', null],
    ['freshness', true],
    ['verification', 'success'],
    ['coverage', {}],
    ['explanation', { private: 'SYNTHETIC_PRIVATE_VALUE' }],
    ['explanation', '   '],
    ['evaluatedAt', 'not-a-date'],
    ['evaluatedAt', undefined],
    ['lastAttemptAt', 123],
    ['lastSuccessfulPointAt', {}],
    ['lastVerifiedAt', 'not-a-date'],
    ['evidenceIds', undefined],
    ['evidenceIds', [1]],
    ['repositoryResourceIds', {}],
    ['providerStates', undefined],
    ['providerStates', {}],
  ])('rejects malformed row field %s=%j without dropping only that row', (key, value) => {
    expect(() =>
      parseProtectionPostureBatches([
        {
          ...response(),
          data: [protectionPosture('vm:valid'), { ...protectionPosture(), [key]: value }],
        },
      ]),
    ).toThrow(ProtectionPostureFormatError);
  });

  it.each([
    ['provider', ''],
    ['source', undefined],
    ['scope', { private: 'SYNTHETIC_PRIVATE_VALUE' }],
    ['jobState', true],
    ['historyCompleteness', 'available'],
    ['permissions', 'granted'],
    ['evidenceIds', [false]],
    ['lastAttemptAt', {}],
    ['lastSuccessAt', 'not-a-date'],
    ['lastVerifiedAt', 123],
    ['verificationExpected', 'false'],
  ])('rejects malformed provider field %s=%j', (key, value) => {
    const row = protectionPosture();
    expect(() =>
      parseProtectionPostureBatches([
        response({ ...row, providerStates: [{ ...row.providerStates[0], [key]: value }] }),
      ]),
    ).toThrow(ProtectionPostureFormatError);
  });

  it.each([
    ['freshnessWindowSeconds', 0],
    ['freshnessWindowSeconds', 0.5],
    ['freshnessWindowSeconds', Infinity],
    ['verificationWindowSeconds', -1],
    ['verificationWindowSeconds', '604800'],
    ['requireVerification', 'false'],
    ['requireVerification', undefined],
  ])('rejects malformed policy field %s=%j', (key, value) => {
    expect(() =>
      parseProtectionPostureBatches([
        { ...response(), policy: { ...protectionPolicy, [key]: value } },
      ]),
    ).toThrow(ProtectionPostureFormatError);
  });

  it('rejects duplicate subjects within or across batches instead of choosing the last health state', () => {
    const protectedRow = protectionPosture();
    const unprotectedRow = { ...protectedRow, state: 'unprotected' };
    expect(() =>
      parseProtectionPostureBatches([{ ...response(), data: [unprotectedRow, protectedRow] }]),
    ).toThrow(ProtectionPostureFormatError);
    expect(() =>
      parseProtectionPostureBatches([response(unprotectedRow), response(protectedRow)]),
    ).toThrow(ProtectionPostureFormatError);
  });

  it.each(['freshnessWindowSeconds', 'verificationWindowSeconds', 'requireVerification'] as const)(
    'rejects mixed %s policies across otherwise valid batches',
    (key) => {
      const changed = { ...protectionPolicy, [key]: key === 'requireVerification' ? false : 3600 };
      expect(() =>
        parseProtectionPostureBatches([
          response(),
          { ...response(protectionPosture('vm:2')), policy: changed },
        ]),
      ).toThrow(ProtectionPostureFormatError);
    },
  );

  it('uses one fixed error without including private row contents or native parser details', () => {
    expect(() =>
      parseProtectionPostureBatches([response({ private: 'SYNTHETIC_PRIVATE_VALUE' })]),
    ).toThrow('The protection posture response format is invalid.');
  });
});
