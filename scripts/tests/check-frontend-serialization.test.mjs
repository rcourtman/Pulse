import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { test } from 'node:test';
import { serializationGraphErrors } from '../check-frontend-serialization.mjs';

function graph(solid = '1.9.16', range = '~1.6.8', seroval = '1.6.8', plugins = '1.6.8') {
  return {
    lockfileVersion: 3,
    packages: {
      'node_modules/solid-js': { version: solid, dependencies: { seroval: range, 'seroval-plugins': range } },
      'node_modules/seroval': { version: seroval },
      'node_modules/seroval-plugins': { version: plugins },
    },
  };
}

test('current complete lock preserves the qualified security override', () => {
  const lock = JSON.parse(readFileSync(new URL('../../frontend-modern/package-lock.json', import.meta.url), 'utf8'));
  assert.deepEqual(serializationGraphErrors(lock), []);
});

test('the exact old qualified tuple is not mistaken for a broken graph', () => {
  assert.deepEqual(serializationGraphErrors(graph('1.9.15', '~1.5.4', '1.6.3', '1.5.5')), []);
});

test('PR 2759 shape rejects both stale serializer dependencies despite npm overrides', () => {
  const errors = serializationGraphErrors(graph('1.9.16', '~1.6.8', '1.6.3', '1.5.5'));
  assert.equal(errors.length, 2);
  assert.match(errors[0], /requires seroval ~1\.6\.8, but resolves 1\.6\.3/);
  assert.match(errors[1], /requires seroval-plugins ~1\.6\.8, but resolves 1\.5\.5/);
});

test('correcting only seroval still rejects the stale plugin', () => {
  const errors = serializationGraphErrors(graph('1.9.16', '~1.6.8', '1.6.8', '1.5.5'));
  assert.equal(errors.length, 1);
  assert.match(errors[0], /requires seroval-plugins/);
});

test('correcting only the plugin still rejects stale seroval', () => {
  const errors = serializationGraphErrors(graph('1.9.16', '~1.6.8', '1.6.3', '1.6.8'));
  assert.equal(errors.length, 1);
  assert.match(errors[0], /requires seroval /);
});

test('the coupled update and later matching patches are not permanently pinned', () => {
  assert.deepEqual(serializationGraphErrors(graph()), []);
  assert.deepEqual(serializationGraphErrors(graph('1.9.17', '~1.6.8', '1.6.9', '1.6.10')), []);
});

test('major and minor mismatches do not satisfy a tilde requirement', () => {
  for (const version of ['2.6.8', '1.7.8', '1.5.99']) {
    assert.equal(serializationGraphErrors(graph('1.9.16', '~1.6.8', version)).length, 1);
  }
});

test('a new Solid cannot borrow the old consumer compatibility exception', () => {
  const errors = serializationGraphErrors(graph('1.9.17', '~1.5.4', '1.6.3', '1.5.5'));
  assert.equal(errors.length, 1);
});

test('a different serializer cannot borrow the exact old override', () => {
  assert.equal(serializationGraphErrors(graph('1.9.15', '~1.5.4', '1.6.4', '1.5.5')).length, 1);
});

test('nested resolution wins over a correct root serializer', () => {
  const lock = graph();
  lock.packages['node_modules/solid-js/node_modules/seroval'] = { version: '1.6.3' };
  const errors = serializationGraphErrors(lock);
  assert.equal(errors.length, 1);
  assert.match(errors[0], /resolves 1\.6\.3/);
});

test('a nested Solid resolves its nearest dependency instead of the root', () => {
  const lock = graph();
  lock.packages['node_modules/fixture/node_modules/solid-js'] = structuredClone(lock.packages['node_modules/solid-js']);
  lock.packages['node_modules/fixture/node_modules/seroval-plugins'] = { version: '1.5.5' };
  assert.equal(serializationGraphErrors(lock).length, 1);
  lock.packages['node_modules/fixture/node_modules/seroval-plugins'] = { version: '1.6.8' };
  assert.deepEqual(serializationGraphErrors(lock), []);
});

test('the old root-only override does not qualify a new nested resolution', () => {
  const lock = graph('1.9.15', '~1.5.4', '1.6.3', '1.5.5');
  lock.packages['node_modules/solid-js/node_modules/seroval'] = { version: '1.6.3' };
  assert.equal(serializationGraphErrors(lock).length, 1);
});

test('missing dependency and consumer entries fail closed', () => {
  for (const name of ['seroval', 'seroval-plugins', 'solid-js']) {
    const lock = graph();
    delete lock.packages[`node_modules/${name}`];
    assert.equal(serializationGraphErrors(lock).length, 1);
  }
});

test('incomplete lock formats fail closed', () => {
  for (const lock of [null, {}, { lockfileVersion: 2, packages: {} }, { lockfileVersion: 3, packages: [] }]) {
    assert.equal(serializationGraphErrors(lock).length, 1);
  }
});

test('unknown ranges require explicit review rather than a made-up semver verdict', () => {
  for (const range of ['^1.6.8', '~1.6', '*', '~1.6.8-beta.1']) {
    const errors = serializationGraphErrors(graph('1.9.16', range));
    assert.equal(errors.length, 2);
  }
  const lock = graph();
  delete lock.packages['node_modules/solid-js'].dependencies.seroval;
  assert.equal(serializationGraphErrors(lock).length, 1);
});

test('prereleases and malformed locked versions cannot claim compatibility', () => {
  for (const version of ['1.6.8-beta.1', '1.6', '01.6.8', 'not-a-version', undefined]) {
    const lock = graph();
    lock.packages['node_modules/seroval'].version = version;
    assert.equal(serializationGraphErrors(lock).length, 1);
  }
  const lock = graph();
  lock.packages['node_modules/solid-js'] = null;
  assert.equal(serializationGraphErrors(lock).length, 1);
});

test('CI fails the graph check before installation without relaxing the audit', () => {
  const workflow = readFileSync(new URL('../../.github/workflows/build-and-test.yml', import.meta.url), 'utf8');
  const frontend = workflow.slice(workflow.indexOf('\n  frontend:'));
  const check = frontend.indexOf('- name: Check Solid serialization dependency edges');
  const install = frontend.indexOf('- name: Install frontend dependencies');
  assert.ok(check >= 0 && check < install);
  assert.match(frontend.slice(check, install), /node --test scripts\/tests\/check-frontend-serialization\.test\.mjs/);
  assert.match(frontend.slice(check, install), /node scripts\/check-frontend-serialization\.mjs/);
  assert.doesNotMatch(frontend.slice(check, install), /continue-on-error/);
  assert.match(frontend, /NPM_AUDIT_REQUIRE_RESULT: \$\{\{ needs\.changes\.outputs\.frontend_deps \}\}/);
  assert.match(frontend, /npm-audit-retry\.sh" all/);
});
