#!/usr/bin/env python3
"""Static guide/source binding, not installed PVE or filesystem recovery."""

from pathlib import Path
import unittest

ROOT = Path(__file__).resolve().parents[2]
HEADING = '### Proxmox LXC filesystem readings with the typed helper\n'
ANCHOR = '#proxmox-lxc-filesystem-readings-with-the-typed-helper'


def plain(text):
    return ' '.join(text.replace('**', '').replace('`', '').split())


class PVEHelperGuidanceTest(unittest.TestCase):
    # A source-root override supports a sensitivity check of the real parent.
    root = ROOT

    def test_existing_guides_and_matrix_reach_one_consistent_boundary(self):
        security = (self.root / 'docs/AGENT_SECURITY.md').read_text()
        general = plain(security.split('## Proxmox Deployment Choices\n', 1)[1].split('\n## ', 1)[0])
        self.assertNotIn("the safe profile uses the typed helper's bounded fixed-shape pct reads", general)
        self.assertIn('Root/full-trust collectors can fall back', general)
        self.assertIn('not usable inside the isolated typed helper', general)
        self.assertGreaterEqual(security.count(ANCHOR), 3)
        row = next(line for line in security.splitlines() if line.startswith('| Proxmox node-local LXC'))
        self.assertIn('command fallback works only outside the helper', row)
        self.assertNotIn('they remain the fallback whenever', row)
        agent = (self.root / 'docs/UNIFIED_AGENT.md').read_text().split(
            '### Inside-Guest Runtime Boundaries\n', 1)[1].split('\n## ', 1)[0]
        self.assertIn('AGENT_SECURITY.md' + ANCHOR, agent)
        self.assertIn('those commands cannot recover readings inside the isolated', plain(agent))
        for name in ('AGENT_SECURITY.md', 'UNIFIED_AGENT.md'):
            self.assertEqual((self.root / 'docs' / name).read_bytes(),
                             (self.root / 'frontend-modern/public/docs' / name).read_bytes())

    def test_partial_and_unknown_readings_do_not_authorise_privilege_or_state_churn(self):
        text = plain((self.root / 'docs/AGENT_SECURITY.md').read_text().split(HEADING, 1)[1].split('\n### ', 1)[0])
        for distinction in ('without pct or lxc-info requests', 'cgroup v2 and /proc',
                            'fallback cannot recover', 'cgroup v1', 'degraded or unavailable',
                            'successful SMART reading does not establish LXC helper health',
                            "reading's status, age and collection path", 'helper versions separate',
                            'Older helper versions', 'without claiming it covers every mount',
                            'Do not remove PrivateNetwork', 'root/sudo fallback, reinstall or restart',
                            'full service/configuration records private', 'operation v2 preserves',
                            'omitted containers cannot refresh old filesystem readings',
                            'Operation v1 remains complete-only', 'explicitly rejects v2',
                            'not an empty healthy snapshot or zero disk usage',
                            'collector-only automatic update cannot replace its helper'):
            with self.subTest(distinction=distinction):
                self.assertIn(distinction, text)
        self.assertNotIn('```', text, 'read existing evidence rather than run a probe')

    def test_current_collector_has_the_described_closed_discovery_and_partial_path(self):
        discover = (self.root / 'internal/hostagent/proxmox_lxc_running.go').read_text()
        collect = (self.root / 'internal/hostagent/proxmox_lxc_filesystems.go').read_text()
        tests = (self.root / 'internal/hostagent/proxmox_lxc_filesystems_test.go').read_text()
        for symbol in ('discoverRunningProxmoxLXCContainers', 'cgroup.controllers', 'filesystemprobe.IsNamespaceInit'):
            self.assertIn(symbol, discover)
        for symbol in ('CollectProxmoxLXCFilesystemsLocalResult', 'ProxmoxLXCCollectionPartial',
                       'OmittedVMIDs', 'result.Degraded = true'):
            self.assertIn(symbol, collect)
        self.assertIn('TestDiscoverRunningProxmoxLXCContainersFindsInitWithoutSockets', tests)
        self.assertIn('pct and lxc-info exist but every call fails', tests)


if __name__ == '__main__':
    unittest.main()
