// Vitest 4 reporter used only by the explicit CI command. Keep the default
// reporter: this receipt contains counts, not errors, stdout or a process exit.
import fs from 'node:fs';
import path from 'node:path';
import { createHash } from 'node:crypto';

const terminalStates = new Set(['passed', 'failed', 'skipped']);
const testStates = ['passed', 'failed', 'skipped', 'pending'];

export default class CIUnitReceipts {
  constructor({ outputFile = process.env.PULSE_UNIT_RECEIPT,
    sourceSha = process.env.PULSE_UNIT_SOURCE_SHA } = {}) {
    if (!outputFile || !/^[a-f0-9]{40}$/.test(sourceSha ?? '')) {
      throw new Error('Unit receipts require an output file and exact source SHA');
    }
    this.outputFile = path.resolve(outputFile);
    this.sourceSha = sourceSha;
  }

  onInit(ctx) {
    if (fs.existsSync(this.outputFile)) {
      throw new Error('Unit receipt destination already exists');
    }
    this.root = path.resolve(ctx.config.root);
    this.modules = new Map();
    this.sharded = Boolean(ctx.config.shard);
    if (this.sharded) {
      // Vitest 4 calls run-start BEFORE the pool shards its inventory. Observe
      // the actual sequencer call once; do not copy/re-run its selection rule
      // or mistake another shard's files for missing results in this one.
      const Sequencer = ctx.config.sequence.sequencer;
      const reporter = this;
      ctx.config.sequence.sequencer = class extends Sequencer {
        async shard(files) {
          const selected = await super.shard(files);
          reporter.onShardSelected(selected);
          return selected;
        }
      };
    }
    this.receipt = {
      schema_version: 1,
      source_sha: this.sourceSha,
      package_lock_sha256: createHash('sha256')
        .update(fs.readFileSync(path.join(this.root, 'package-lock.json'))).digest('hex'),
      shard: ctx.config.shard ?? { index: 1, count: 1 },
      inventory_observed: false,
      selection_observed: false,
      test_run_end_observed: false,
      test_run_reason: null,
      unhandled_error_count: null,
    };
    this.write();
  }

  onTestRunStart(specifications) {
    const modules = new Map();
    for (const spec of specifications) {
      const module = path.relative(this.root, spec.moduleId);
      if (!module || path.isAbsolute(module) || module === '..'
        || module.startsWith(`..${path.sep}`) || !spec.taskId || modules.has(spec.taskId)) {
        throw new Error('Invalid or duplicate unit inventory entry');
      }
      modules.set(spec.taskId, {
        id: spec.taskId,
        module: module.split(path.sep).join('/'),
        project: spec.project.name,
        pool: spec.pool,
        state: 'unobserved',
        tests: null,
        module_error_count: null,
      });
    }
    this.discovered = modules;
    this.modules = new Map();
    this.receipt.inventory_observed = true;
    this.receipt.discovered_module_count = modules.size;
    this.receipt.selection_observed = false;
    this.receipt.test_run_end_observed = false;
    this.receipt.test_run_reason = null;
    this.receipt.unhandled_error_count = null;
    if (this.sharded) this.write();
    else this.onShardSelected(specifications);
  }

  onShardSelected(specifications) {
    const selected = new Map();
    for (const spec of specifications) {
      const entry = this.discovered?.get(spec.taskId);
      if (!entry || selected.has(spec.taskId)) {
        throw new Error('Shard selection is absent from the discovered unit inventory');
      }
      selected.set(spec.taskId, entry);
    }
    this.modules = selected;
    this.receipt.selection_observed = true;
    this.write();
  }

  observe(module) {
    const entry = this.modules.get(module.id);
    if (!entry) throw new Error('Unit result is absent from the selected inventory');
    const state = module.state();
    if (!terminalStates.has(state) && state !== 'pending' && state !== 'queued') {
      throw new Error('Unknown unit module state');
    }
    const tests = Object.fromEntries(testStates.map(state => [state, 0]));
    for (const test of module.children.allTests()) {
      const result = test.result().state;
      if (!testStates.includes(result)) throw new Error('Unknown unit test state');
      tests[result]++;
    }
    this.modules.set(module.id, {
      ...entry, state, tests, module_error_count: module.errors().length,
    });
  }

  onTestModuleEnd(module) {
    this.observe(module);
    this.write();
  }

  onTestRunEnd(modules, unhandledErrors, reason) {
    if (!['passed', 'failed', 'interrupted'].includes(reason)) {
      throw new Error('Unknown unit run end reason');
    }
    for (const module of modules) this.observe(module);
    this.receipt.test_run_end_observed = true;
    this.receipt.test_run_reason = reason;
    this.receipt.unhandled_error_count = unhandledErrors.length;
    this.write();
  }

  write() {
    const modules = [...this.modules.values()].sort((a, b) =>
      a.module.localeCompare(b.module) || a.id.localeCompare(b.id));
    const unobserved = modules.filter(module => !terminalStates.has(module.state))
      .map(module => module.id);
    const receipt = {
      ...this.receipt,
      // A run-end callback is not an observed process exit or a passing suite.
      collection_complete: this.receipt.inventory_observed && this.receipt.selection_observed
        && modules.length > 0
        && this.receipt.test_run_end_observed
        && this.receipt.test_run_reason !== 'interrupted' && unobserved.length === 0,
      unobserved_modules: unobserved,
      modules,
    };
    fs.mkdirSync(path.dirname(this.outputFile), { recursive: true });
    const temporary = `${this.outputFile}.${process.pid}.tmp`;
    let owned = false;
    try {
      fs.writeFileSync(temporary, `${JSON.stringify(receipt, null, 2)}\n`,
        { flag: 'wx', mode: 0o600 });
      owned = true;
      fs.renameSync(temporary, this.outputFile);
      owned = false;
    } finally {
      if (owned) fs.unlinkSync(temporary);
    }
  }
}
