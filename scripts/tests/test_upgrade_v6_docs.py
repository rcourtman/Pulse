#!/usr/bin/env python3
"""Keep the v6 migration entry point on safe, deployment-owned update paths.

These are documentation and actual link-target controls, not native upgrade,
rollback, collector recovery or private-runtime acceptance.
"""
from pathlib import Path
import re
import sys
import unittest

ROOT = Path(__file__).resolve().parents[2]
GUIDE = ROOT / 'docs/UPGRADE_v6.md'
sys.path.insert(0, str(ROOT / 'scripts'))
from check_public_docs import anchors_for, split_target  # noqa: E402


def guide():
    return GUIDE.read_text(encoding='utf-8')


def section(heading, level='##'):
    return guide().split(f'{level} {heading}\n', 1)[1].split(f'\n{level} ', 1)[0]


def prose(text):
    return ' '.join(text.split())


class UpgradeV6DocsTest(unittest.TestCase):
    def test_preparation_uses_full_state_not_configuration_export_or_live_copy(self):
        before = prose(section('Before You Upgrade'))
        for boundary in ('not a full-state backup', 'history and agent enrolment state',
                         'matching encryption/signing keys', 'MIGRATION.md#full-state-recovery',
                         'do not copy live data', 'every active store', 'existing service and data/config paths',
                         'does not require first-time setup', 'interrupt monitoring and alert delivery',
                         'independent coverage', 'Do not update during a backup, freeze/thaw'):
            with self.subTest(boundary=boundary):
                self.assertIn(boundary, before)

    def test_update_routes_do_not_copy_unqualified_mutating_shortcuts(self):
        routes = section('Upgrade Paths')
        self.assertNotRegex(routes, r'```(?:bash|sh|shell)\b')
        for boundary in ('INSTALL.md#manual-update', "helper's ownership", 'inside the existing Pulse LXC',
                         'not on the Proxmox host', 'Do not execute an absent or unknown helper',
                         'default installation paths', 'AUTO_UPDATE.md#a-helper-selects-a-helm-chart-release',
                         'server-installer flow is for Community', 'do not replace a private Pro runtime',
                         'separate from server updates', 'UNIFIED_AGENT.md#private-file-installation-linux-macos-and-nas',
                         'do not use the GitHub server installer for `pulse-agent`'):
            with self.subTest(boundary=boundary):
                self.assertIn(boundary, prose(routes))

    def test_docker_requires_configured_image_and_only_pulse_recreation(self):
        docker = prose(section('Docker', '###'))
        for boundary in ('DOCKER.md#-updates', 'Change the configured image', 'exact published tag first',
                         'recreate only the Pulse service', 'mounted data, credentials',
                         'does not change a hardcoded Compose image or the running container',
                         'Keep a private Pro image private', 'Do not run `docker compose down`',
                         'remove volumes', 'recreate unrelated services'):
            with self.subTest(boundary=boundary):
                self.assertIn(boundary, docker)

    def test_kubernetes_preserves_existing_values_secrets_and_volume_scope(self):
        helm = prose(section('Kubernetes (Helm)', '###').split('## Post-Upgrade Checklist', 1)[0])
        for boundary in ('KUBERNETES.md#1-deploying-the-pulse-server',
                         'existing release, namespace, values and persistent volumes',
                         'published chart and server image separately', 'running server version or edition',
                         'private Pro image configuration and Secret references',
                         'do not paste credentials into Helm command arguments', 'Secret YAML'):
            with self.subTest(boundary=boundary):
                self.assertIn(boundary, helm)

    def test_post_upgrade_does_not_create_notification_or_diagnostic_traffic(self):
        post = prose(section('Post-Upgrade Checklist').split('### Failed or interrupted upgrade', 1)[0])
        self.assertNotRegex(post.lower(), r'confirm notifications.*send a test')
        for boundary in ('passive observations', 'API.md#-authentication', 'never a token or session cookie',
                         'running server version and edition', 'not completed installation',
                         'ordinary polling', 'original source/time', 'does not prove every collector recovered',
                         'ordinary notification delivery', 'do not send a Test, trigger an alert or replay a queue',
                         'Check agents individually', 'previously active services/timers',
                         'every-covered-filesystem writes', 'An OK backup task is not that proof'):
            with self.subTest(boundary=boundary):
                self.assertIn(boundary, post)

    def test_uncertain_update_retains_stop_evidence_and_does_not_repeat(self):
        failure = prose(section('Failed or interrupted upgrade', '###').split('## v5 to v6 Operator FAQ', 1)[0])
        for boundary in ('running version, edition and service health', 'original attempt time/timezone',
                         'broken pipe does not establish', 'later boot is a new run',
                         'AUTO_UPDATE.md#update-failed', 'AUTO_UPDATE.md#pause-further-unattended-attempts-without-stopping-pulse',
                         'without stopping a working Pulse server', 'Do not start, cancel or repeat',
                         'Do not repeat the update, reboot, restore, run Diagnostics or delete rollback backups',
                         'AUTO_UPDATE.md#manual-rollback', 'matching data and keys',
                         'reversible copies of the failed state', 'not a data rollback',
                         'TROUBLESHOOTING.md#-getting-help', 'not backups, full logs or deployment configuration'):
            with self.subTest(boundary=boundary):
                self.assertIn(boundary, failure)

    def test_historical_v5_downgrade_is_not_a_live_migrated_data_recipe(self):
        faq = guide().split('### Can I keep Pulse v5 stable while I test Pulse v6?', 1)[1].split('## Migration Notes (v6)', 1)[0]
        self.assertNotRegex(faq, r'```(?:bash|sh|shell)\b')
        for boundary in ('isolated recovery instance', 'no access to monitored systems or notification destinations',
                         'do not start a second connected copy', 'data migrated by v6',
                         'v5.1.36, is end-of-life', 'not a supported recovery target', 'same edition',
                         'INSTALL.md#rollback', 'live migrated data', 'version-selection behaviour does not prove',
                         'Do not blindly run another updater or downgrade'):
            with self.subTest(boundary=boundary):
                self.assertIn(boundary, prose(faq))

    def test_old_signer_warning_preserves_trust_without_reinstall_instruction(self):
        routes = prose(section('Upgrade Paths'))
        for boundary in ('historical Pulse update signer was not recovered', 'trust-continuity limit',
                         'not an instruction to reinstall or bypass signature verification',
                         'explicit verified trust migration', 'identity, data and private edition'):
            self.assertIn(boundary, routes)
        self.assertNotIn('plan a manual reinstall', routes)

    def test_retired_relay_is_not_offered_as_a_new_upgrade_purchase(self):
        licensing = prose(section('License and Entitlements', '###'))
        self.assertNotIn('Community / Relay / Pro plan model', licensing)
        self.assertNotIn('Relay raises history to 14 days', licensing)
        for boundary in ('Relay is no longer sold', 'existing Relay subscribers receive Pro features',
                         'current price while their subscription continues', 'private runtime'):
            self.assertIn(boundary, licensing)

    def test_connected_local_update_and_recovery_targets_exist(self):
        links = re.findall(r'\[[^\]]*\]\(([^)]+)\)', guide())
        required = ('INSTALL.md#manual-update', 'INSTALL.md#rollback', 'DOCKER.md#-updates',
                    'MIGRATION.md#full-state-recovery', 'AUTO_UPDATE.md#manual-rollback',
                    'AUTO_UPDATE.md#update-failed', 'KUBERNETES.md#1-deploying-the-pulse-server',
                    '#post-upgrade-checklist', '#failed-or-interrupted-upgrade')
        for target in required:
            with self.subTest(target=target):
                self.assertIn(target, links)
                path, anchor = split_target(target)
                resolved = (GUIDE.parent / path).resolve() if path else GUIDE
                self.assertTrue(resolved.is_file())
                # The existing docs validator defers emoji-derived leading-hyphen
                # aliases to the consuming browser; ordinary anchors remain exact.
                if not anchor.startswith('-'):
                    self.assertIn(anchor, anchors_for(resolved))

    def test_shipped_guide_is_byte_identical(self):
        self.assertEqual(GUIDE.read_bytes(), (ROOT / 'frontend-modern/public/docs/UPGRADE_v6.md').read_bytes())


if __name__ == '__main__':
    unittest.main()
