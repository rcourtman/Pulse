"""Opt-in real Chromium cleanup proof. Run through pulse-heavy-run, not unit discovery.

Uses synthetic cookies and an offline page; does not qualify Docker teardown,
Pulse authentication, sharing, or the Playwright Test reporter lifecycle.
"""
import json
import os
from pathlib import Path
import signal
import subprocess
import tempfile
import time
import unittest

SUPERVISOR = Path(__file__).with_name('owned-run.py')
PLAYWRIGHT = (SUPERVISOR.parent.parent / 'node_modules/playwright/index.mjs').as_uri()
LAUNCH = '''import importlib.util, sys
from pathlib import Path
s = importlib.util.spec_from_file_location('owned', sys.argv[1])
m = importlib.util.module_from_spec(s); s.loader.exec_module(m)
sys.exit(m.supervise(['node', '--input-type=module', '-e', sys.argv[3]], Path(sys.argv[2])))
'''
BROWSER = r'''
import { chromium } from 'PLAYWRIGHT_MODULE';
import fs from 'node:fs';
const env = process.env;
const results = env.PULSE_E2E_RESULTS_DIR;
const report = env.PULSE_E2E_REPORT_DIR;
fs.mkdirSync(results, { recursive: true });
fs.mkdirSync(report, { recursive: true });
const server = await chromium.launchServer({ headless: true,
  handleSIGINT: false, handleSIGTERM: false, handleSIGHUP: false });
const browser = await chromium.connect(server.wsEndpoint());
const context = await browser.newContext({ recordVideo: { dir: results } });
await context.addCookies([{ name: 'fixture', value: 'synthetic-only', domain: 'example.invalid', path: '/' }]);
await context.tracing.start({ screenshots: true, snapshots: true });
const page = await context.newPage();
await page.setContent('<h1>Offline interruption qualification</h1>');
await context.storageState({ path: env.PULSE_E2E_COOKIE_STATE_PATH });
await page.screenshot({ path: results + '/before.png' });
let stopping = false;
process.on('SIGTERM', async () => {
  if (stopping) return;
  stopping = true;
  try {
    // Exercise genuine browser writers after interruption, before reaping.
    await new Promise(resolve => setTimeout(resolve, 200));
    await page.screenshot({ path: results + '/late.png' });
    await context.storageState({ path: env.PULSE_E2E_COOKIE_STATE_PATH });
    await context.tracing.stop({ path: results + '/trace.zip' });
    const video = page.video();
    await context.close();
    await video.saveAs(results + '/finished.webm');
    const videos = fs.readdirSync(results).filter(name => name.endsWith('.webm'));
    if (!videos.length || videos.some(name => fs.statSync(results + '/' + name).size === 0)) throw Error('no finished video');
    fs.writeFileSync(report + '/summary.json', JSON.stringify({ videos: videos.length }));
    await browser.close();
    await server.close();
    fs.writeFileSync(env.PROOF_ROOT + '/closed', 'browser closed; late screenshot, storage, trace, video and summary written');
    process.exit(0);
  } catch (error) { console.error(error); process.exit(1); }
});
fs.writeFileSync(env.PROOF_ROOT + '/ready', JSON.stringify({ pid: server.process().pid,
  paths: [env.PULSE_E2E_COOKIE_STATE_PATH, report, results] }));
'''.replace('PLAYWRIGHT_MODULE', PLAYWRIGHT)


class BrowserCleanupTest(unittest.TestCase):
    def test_catchable_signals(self):
        for sig in (signal.SIGTERM, signal.SIGINT, signal.SIGHUP):
            with self.subTest(signal=sig.name), tempfile.TemporaryDirectory(prefix='pulse-browser-proof-') as temp:
                root = Path(temp)
                neighbour = root / 'unowned-cookie'
                neighbour.write_text('keep')
                process = subprocess.Popen(['python3', '-c', LAUNCH, str(SUPERVISOR), temp, BROWSER],
                    env={**os.environ, 'PROOF_ROOT': temp, 'PULSE_E2E_COOKIE_STATE_PATH': str(neighbour)})
                try:
                    deadline = time.monotonic() + 30
                    while not (root / 'ready').exists():
                        self.assertIsNone(process.poll(), 'browser exited before ready')
                        self.assertLess(time.monotonic(), deadline, 'browser startup timeout')
                        time.sleep(.05)
                    receipt = json.loads((root / 'ready').read_text())
                    self.assertTrue(Path(receipt['paths'][0]).is_file())
                    self.assertTrue(Path('/proc', str(receipt['pid'])).exists())
                    process.send_signal(sig)
                    self.assertEqual(process.wait(timeout=20), 128 + sig)
                    self.assertTrue((root / 'closed').exists(), 'real browser late writes and close must finish')
                    self.assertFalse(Path('/proc', str(receipt['pid'])).exists())
                    for index, value in enumerate(receipt['paths']):
                        owned = Path(value).parent if index == 0 else Path(value)
                        self.assertFalse(owned.exists(), str(owned))
                    time.sleep(.3)
                    self.assertFalse(Path(receipt['paths'][2]).exists(), 'no surviving writer recreation')
                    self.assertEqual(neighbour.read_text(), 'keep')
                finally:
                    if process.poll() is None:
                        process.send_signal(signal.SIGTERM)
                        process.wait(timeout=20)


if __name__ == '__main__':
    unittest.main()
