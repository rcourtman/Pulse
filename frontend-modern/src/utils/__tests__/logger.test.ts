import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

type LoggerModule = typeof import('@/utils/logger');

describe('logger', () => {
  let logSpy: ReturnType<typeof vi.spyOn>;
  let warnSpy: ReturnType<typeof vi.spyOn>;
  let errorSpy: ReturnType<typeof vi.spyOn>;

  beforeEach(() => {
    logSpy = vi.spyOn(console, 'log').mockImplementation(() => {});
    warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {});
    errorSpy = vi.spyOn(console, 'error').mockImplementation(() => {});
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  describe('in development (DEV=true)', () => {
    let logger: LoggerModule['logger'];
    let logError: LoggerModule['logError'];

    beforeEach(async () => {
      vi.resetModules();
      vi.stubEnv('DEV', true);
      const mod = await import('@/utils/logger');
      logger = mod.logger;
      logError = mod.logError;
    });

    afterEach(() => {
      vi.unstubAllEnvs();
      vi.resetModules();
    });

    it('debug logs with a [DEBUG] prefix via console.log', () => {
      logger.debug('booting up', { phase: 1 });

      expect(logSpy).toHaveBeenCalledWith('%s', '[DEBUG] booting up', { phase: 1 });
    });

    it('debug omits data gracefully (defaults to empty string)', () => {
      logger.debug('no data');

      expect(logSpy).toHaveBeenCalledWith('%s', '[DEBUG] no data', '');
    });

    const debugDataCases: Array<[string, unknown, unknown]> = [
      ['object', { a: 1 }, { a: 1 }],
      ['array', [1, 2], [1, 2]],
      ['number 0', 0, 0],
      ['false boolean', false, false],
      ['empty string', '', ''],
      ['null becomes empty string', null, ''],
      ['undefined becomes empty string', undefined, ''],
      ['NaN', NaN, NaN],
    ];

    it.each(debugDataCases)(
      'debug passes %s through as the third console.log arg',
      (_label, data, expected) => {
        logger.debug('msg', data);

        expect(logSpy).toHaveBeenCalledWith('%s', '[DEBUG] msg', expected);
      },
    );

    it('info always logs in dev even without keywords', () => {
      logger.info('just info');

      expect(logSpy).toHaveBeenCalledWith('%s', '[INFO] just info', '');
    });

    it('info passes data through in dev', () => {
      logger.info('booting', { phase: 1 });

      expect(logSpy).toHaveBeenCalledWith('%s', '[INFO] booting', { phase: 1 });
    });

    it('warn always logs via console.warn with [WARN] prefix', () => {
      logger.warn('careful', { x: 1 });

      expect(warnSpy).toHaveBeenCalledWith('%s', '[WARN] careful', { x: 1 });
    });

    it('warn defaults data to empty string when omitted', () => {
      logger.warn('careful');

      expect(warnSpy).toHaveBeenCalledWith('%s', '[WARN] careful', '');
    });

    it('error always logs via console.error with [ERROR] prefix', () => {
      const err = new Error('boom');
      logger.error('something broke', err);

      expect(errorSpy).toHaveBeenCalledWith('%s', '[ERROR] something broke', err);
    });

    it('error defaults error to empty string when omitted', () => {
      logger.error('something broke');

      expect(errorSpy).toHaveBeenCalledWith('%s', '[ERROR] something broke', '');
    });

    it('logError is an alias for logger.error', () => {
      expect(logError).toBe(logger.error);

      const err = new Error('x');
      logError('aliased', err);

      expect(errorSpy).toHaveBeenCalledWith('%s', '[ERROR] aliased', err);
    });
  });

  describe('in production (DEV=false)', () => {
    let logger: LoggerModule['logger'];

    beforeEach(async () => {
      vi.resetModules();
      vi.stubEnv('DEV', false);
      const mod = await import('@/utils/logger');
      logger = mod.logger;
    });

    afterEach(() => {
      vi.unstubAllEnvs();
      vi.resetModules();
    });

    it('debug does not log in production', () => {
      logger.debug('hello', { a: 1 });

      expect(logSpy).not.toHaveBeenCalled();
    });

    const infoKeywordCases: Array<[string, boolean]> = [
      ['connection established', true],
      ['the operation failed', true],
      ['an error occurred', true],
      ['established', true],
      ['failed', true],
      ['error', true],
      ['all systems nominal', false],
      ['just info', false],
    ];

    it.each(infoKeywordCases)('info in production logs for %j? %s', (message, shouldLog) => {
      logger.info(message);

      if (shouldLog) {
        expect(logSpy).toHaveBeenCalledWith('%s', `[INFO] ${message}`, '');
      } else {
        expect(logSpy).not.toHaveBeenCalled();
      }
    });

    // The keyword check uses String.includes, which is case-sensitive. Capitalized
    // variants of the trigger words are therefore NOT surfaced in production.
    const capitalizedKeywordCases: Array<[string]> = [['Established'], ['Failed'], ['Error']];

    it.each(capitalizedKeywordCases)(
      'info in production does NOT log for capitalized keyword %j (case-sensitive match, suspected bug)',
      (message) => {
        logger.info(message);

        expect(logSpy).not.toHaveBeenCalled();
      },
    );

    it('info in production passes data through when a keyword matches', () => {
      logger.info('connection established', { id: 7 });

      expect(logSpy).toHaveBeenCalledWith('%s', '[INFO] connection established', { id: 7 });
    });

    it('warn still logs in production', () => {
      logger.warn('careful');

      expect(warnSpy).toHaveBeenCalledWith('%s', '[WARN] careful', '');
    });

    it('error still logs in production', () => {
      const err = new Error('boom');
      logger.error('broke', err);

      expect(errorSpy).toHaveBeenCalledWith('%s', '[ERROR] broke', err);
    });
  });
});
