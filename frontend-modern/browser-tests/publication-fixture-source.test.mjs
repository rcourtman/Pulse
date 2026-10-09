import { describe, expect, it } from 'vitest';
import source from './publication-fixture-source.cjs';

const sha = 'a'.repeat(40);
describe('publication fixture source attribution', () => {
  it('reads the actual Git head by default', () => {
    expect(source.publicationSourceIdentity([], () => sha + '\n')).toEqual({
      source_sha: sha,
      source_identity: 'git-head',
    });
  });
  it('never silently substitutes a supplied identity after a failed Git read', () => {
    expect(() =>
      source.publicationSourceIdentity([], () => {
        throw new Error('Git unavailable');
      }),
    ).toThrow('Git unavailable');
  });
  it('explicitly attributes an isolated copy without trying to access an unmounted Git store', () => {
    expect(
      source.publicationSourceIdentity(['--source-sha', sha], () => {
        throw new Error('must not read outside the isolated copy');
      }),
    ).toEqual({ source_sha: sha, source_identity: 'supplied-for-isolated-copy' });
  });
  it.each([
    [],
    ['--source-sha'],
    ['--source-sha', 'a'.repeat(39)],
    ['--source-sha', 'A'.repeat(40)],
    ['--source-sha', 'main'],
    ['--source-sha', sha, '--extra'],
    ['--unknown', sha],
  ])('rejects malformed identity or arguments %j', (...args) => {
    expect(() => source.publicationSourceIdentity(args, () => 'main')).toThrow();
  });
});
