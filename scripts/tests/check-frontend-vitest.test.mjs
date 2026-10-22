import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { mock, test } from 'node:test';
import { main, vitestGraphErrors } from '../check-frontend-vitest.mjs';

// Version-only fixtures test admission, not acquisition or compatibility of new
// package bytes. The complete current manifest/lock and CLI are checked as well.
function graph(version = '4.1.12') {
  const devDependencies = { vitest: `^${version}`, '@vitest/coverage-v8': `^${version}` };
  return {
    manifest: { devDependencies },
    lock: { lockfileVersion: 3, packages: {
      '': { devDependencies: { ...devDependencies } },
      ...Object.fromEntries(['vitest', '@vitest/mocker', '@vitest/coverage-v8']
        .map((name) => [`node_modules/${name}`, { version }])),
    } },
  };
}

const errors = ({ manifest, lock }) => vitestGraphErrors(manifest, lock);

test('current complete frontend graph and executable CLI retain the security floor', () => {
  const manifest = JSON.parse(readFileSync(new URL('../../frontend-modern/package.json', import.meta.url), 'utf8'));
  const lock = JSON.parse(readFileSync(new URL('../../frontend-modern/package-lock.json', import.meta.url), 'utf8'));
  assert.deepEqual(vitestGraphErrors(manifest, lock), []);
  const output = mock.method(console, 'log', () => {});
  try {
    assert.equal(main(), 0);
    assert.match(output.mock.calls[0].arguments[0], /exact runner\/mocker\/coverage alignment retained/);
  } finally {
    output.mock.restore();
  }
});

test('patched locked bytes cannot excuse a declaration below the current advisory floor', () => {
  for (const name of ['vitest', '@vitest/coverage-v8']) {
    const fixture = graph();
    fixture.manifest.devDependencies[name] = '^4.1.11';
    fixture.lock.packages[''].devDependencies[name] = '^4.1.11';
    assert.match(errors(fixture).join('\n'), /at or above \^4\.1\.12/);
  }
});

test('reviewed declarations accept a later matching patched graph, not a historical text pin', () => {
  const fixture = graph('4.1.13');
  for (const name of ['vitest', '@vitest/coverage-v8']) {
    fixture.manifest.devDependencies[name] = '^4.1.12';
    fixture.lock.packages[''].devDependencies[name] = '^4.1.12';
  }
  assert.deepEqual(errors(fixture), []);
});

test('later stable patches and minors on the reviewed major are eligible', () => {
  for (const version of ['4.1.12', '4.1.13', '4.2.0']) {
    assert.deepEqual(errors(graph(version)), [], version);
  }
});

test('unsafe, broad, malformed and unreviewed declarations fail closed for both consumers', () => {
  for (const name of ['vitest', '@vitest/coverage-v8']) {
    for (const range of ['^4.1.11', '^4.1.10', '^4.0.99', '^5.0.0', '^4.1.12-beta.1', '*', '>=4.1.12',
      '^4.1.12 || ^3.0.0', '~4.1.12', '4.1.12', '^04.1.12', '^4.01.12', '^4.1.12\n',
      '^4.1.9007199254740992', null, undefined]) {
      const fixture = graph();
      fixture.manifest.devDependencies[name] = range;
      fixture.lock.packages[''].devDependencies[name] = range;
      assert.notDeepEqual(errors(fixture), [], `${name}: ${range}`);
    }
  }
});

test('locked companions must be stable, patched and on the reviewed major', () => {
  for (const name of ['vitest', '@vitest/mocker', '@vitest/coverage-v8']) {
    for (const version of ['4.1.11', '4.1.10', '4.0.99', '3.9.99', '5.0.0', '4.1.12-beta.1', '4.1.12+build',
      '4.1', '04.1.12', '4.01.12', '4.1.12\n', '4.1.9007199254740992', null, undefined]) {
      const fixture = graph();
      fixture.lock.packages[`node_modules/${name}`] = { version };
      assert.notDeepEqual(errors(fixture), [], `${name}: ${version}`);
    }
  }
});

test('even safe companion version drift is not exact runner alignment', () => {
  for (const name of ['@vitest/mocker', '@vitest/coverage-v8']) {
    const fixture = graph();
    fixture.lock.packages[`node_modules/${name}`].version = '4.1.13';
    assert.match(errors(fixture).join('\n'), /must exactly match/);
  }
});

test('nested unsafe, missing-version or mismatched copies cannot hide behind a patched root', () => {
  for (const name of ['vitest', '@vitest/mocker', '@vitest/coverage-v8']) {
    for (const version of ['4.1.11', '4.1.13', '5.0.0', undefined]) {
      const fixture = graph();
      fixture.lock.packages[`node_modules/tool/node_modules/${name}`] = { version };
      assert.notDeepEqual(errors(fixture), [], `${name}: ${version}`);
    }
  }
  const fixture = graph();
  fixture.lock.packages['node_modules/tool/node_modules/@vitest/mocker'] = { version: '4.1.12' };
  assert.deepEqual(errors(fixture), []);
});

test('a missing package or nested-only runner is not a complete graph', () => {
  for (const name of ['vitest', '@vitest/mocker', '@vitest/coverage-v8']) {
    const fixture = graph();
    delete fixture.lock.packages[`node_modules/${name}`];
    assert.notDeepEqual(errors(fixture), [], name);
  }
  const fixture = graph();
  fixture.lock.packages['node_modules/tool/node_modules/vitest'] = fixture.lock.packages['node_modules/vitest'];
  delete fixture.lock.packages['node_modules/vitest'];
  assert.notDeepEqual(errors(fixture), []);
});

test('manifest floors, locked runner and root lock declarations must agree', () => {
  for (const name of ['vitest', '@vitest/coverage-v8']) {
    const fixture = graph();
    fixture.manifest.devDependencies[name] = '^4.1.13';
    fixture.lock.packages[''].devDependencies[name] = '^4.1.13';
    assert.match(errors(fixture).join('\n'), /below the declared/);
    const stale = graph();
    delete stale.lock.packages[''].devDependencies[name];
    assert.match(errors(stale).join('\n'), /root lock declaration/);
  }
});

test('incomplete or malformed graph inputs fail closed', () => {
  for (const lock of [null, {}, { lockfileVersion: 2, packages: {} }, { lockfileVersion: 3, packages: [] }]) {
    assert.notDeepEqual(vitestGraphErrors(graph().manifest, lock), []);
  }
  for (const manifest of [null, {}, { devDependencies: null }]) {
    assert.notDeepEqual(vitestGraphErrors(manifest, graph().lock), []);
  }
});

test('ordinary CI checks this exact guard before installation and retains the complete audit', () => {
  const workflow = readFileSync(new URL('../../.github/workflows/build-and-test.yml', import.meta.url), 'utf8');
  const frontend = workflow.slice(workflow.indexOf('\n  frontend:'));
  const check = frontend.indexOf('- name: Check reviewed Vitest dependency alignment');
  const install = frontend.indexOf('- name: Install frontend dependencies');
  assert.ok(check >= 0 && check < install);
  assert.match(frontend.slice(check, install), /node --test scripts\/tests\/check-frontend-vitest\.test\.mjs/);
  assert.match(frontend.slice(check, install), /node scripts\/check-frontend-vitest\.mjs/);
  assert.doesNotMatch(frontend.slice(check, install), /continue-on-error|\n        if:/);
  assert.match(frontend, /NPM_AUDIT_REQUIRE_RESULT: \$\{\{ needs\.changes\.outputs\.frontend_deps \}\}/);
  assert.match(frontend, /npm-audit-retry\.sh" all/);
  const securityTest = readFileSync(new URL('../../frontend-modern/src/security/__tests__/dependencySecurity.test.ts', import.meta.url), 'utf8');
  assert.match(securityTest, /scripts\/check-frontend-vitest\.mjs/);
  assert.match(securityTest, /execFileSync\(process\.execPath, \[check\]/);
});
