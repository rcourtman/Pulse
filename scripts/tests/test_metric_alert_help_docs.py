#!/usr/bin/env python3
"""Keep lingering-alert help tied to live evaluation, not a retained breach."""

from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[2]
HEADING = '#### Usage alert stays open after the reading falls\n'
ANCHOR = '#usage-alert-stays-open-after-the-reading-falls'


class MetricAlertHelpDocsTest(unittest.TestCase):
    def setUp(self):
        self.source = (ROOT / 'docs/TROUBLESHOOTING.md').read_text()
        self.guide = ' '.join(self.source.split(HEADING, 1)[1].split('\n#### ', 1)[0].split())
        self.presentation = (ROOT / 'frontend-modern/src/features/alerts/metricAlertPresentation.ts').read_text()

    def test_workflow_entry_reaches_one_section_in_the_shipped_copy(self):
        self.assertEqual(self.source.count(HEADING), 1)
        self.assertIn(ANCHOR, self.source.split('## ⚡ Quick Fixes', 1)[0])
        self.assertEqual((ROOT / 'docs/TROUBLESHOOTING.md').read_bytes(),
                         (ROOT / 'frontend-modern/public/docs/TROUBLESHOOTING.md').read_bytes())

    def test_named_states_and_units_follow_the_actual_card_presenter(self):
        for label in ('Alert still open', 'Recovering', 'No recent reading'):
            with self.subTest(label=label):
                self.assertIn(f"phaseLabel = '{label}'", self.presentation)
                self.assertIn(f'**{label}**', self.guide)
        for label in ('averaged', 'latest', 'Last reading at or above'):
            with self.subTest(label=label):
                self.assertIn(label, self.presentation)
                self.assertIn(f'**{label}**', self.guide)
        card = (ROOT / 'frontend-modern/src/features/alerts/AlertOverviewAlertCard.tsx').read_text()
        self.assertIn('getMetricAlertPresentation(props.alert', card)
        self.assertIn('not current usage', self.guide)
        self.assertIn('match the filesystem too', self.guide)

    def test_clear_rule_example_is_not_a_universal_or_immediate_promise(self):
        self.assertIn('90% trigger and 85% clear, 88% can still hold', self.guide)
        self.assertIn('not the saved rule for every resource', self.guide)
        self.assertIn('elapsed wall-clock time alone does not prove', self.guide)
        self.assertIn('fresh eligible observations', self.guide)
        anchor = 'CONFIGURATION.md#metric-thresholds-off-and-inheritance'
        self.assertIn(anchor, self.guide)
        self.assertIn('### Metric thresholds, Off and inheritance',
                      (ROOT / 'docs/CONFIGURATION.md').read_text())
        # The producer retains breach fields during a hold; only live status moves.
        producer = (ROOT / 'internal/alerts/canonical_metric.go').read_text()
        hold = producer.split('if !triggered && primary == "" {', 1)[1].split('return true', 1)[0]
        self.assertIn('existingAlert.MetricStatus = metricStatus', hold)
        self.assertNotIn('existingAlert.Value =', hold)
        self.assertNotIn('existingAlert.LastSeen =', hold)

    def test_missing_data_attention_and_delivery_are_not_measured_recovery(self):
        for boundary in (
            'retained evidence, not a healthy zero',
            'missing detail does not establish a current breach',
            'not required for a threshold alert to recover',
            'retained failed notification can outlive its incident',
            '#recover-retained-delivery-failures',
            'Unknown details are valid', 'Add only missing evidence',
            'do not rerun a backup, create load, restart, change thresholds',
            'disable monitoring or retry notifications', 'Do not delete alert or queue data',
            'not proof that the workload recovered',
        ):
            with self.subTest(boundary=boundary):
                self.assertIn(boundary, self.guide)
        self.assertNotIn('```', self.guide)

    def test_threshold_marker_help_follows_both_current_consumers(self):
        source = (ROOT / 'docs/CONFIGURATION.md').read_text()
        heading = '### Read active-alert markers\n'
        self.assertEqual(source.count(heading), 1)
        self.assertIn('[active-alert markers](#read-active-alert-markers)', source)
        self.assertEqual(source.encode(),
                         (ROOT / 'frontend-modern/public/docs/CONFIGURATION.md').read_bytes())
        guide = ' '.join(source.split(heading, 1)[1].split('\n### ', 1)[0].split())
        for file in ('AlertResourceTableRow.tsx', 'AlertResourceTableMobile.tsx'):
            consumer = (ROOT / 'frontend-modern/src/components/Alerts' / file).read_text()
            self.assertIn('title="Active alert"', consumer)
            self.assertIn('hasActiveAlert(', consumer)
        self.assertIn('current alert list contains an open alert for that row and metric', guide)
        self.assertIn('not a new reading, another threshold', guide)
        self.assertIn('No dot is not proof that the workload is healthy', guide)
        self.assertIn('Recent delivery activity', guide)
        self.assertIn('#usage-alert-stays-open-after-the-reading-falls', guide)
        self.assertIn('#test-succeeds-but-real-alerts-are-missing', guide)
        self.assertIn('Do not create load, change thresholds or send a Test', guide)
        self.assertNotIn('```', guide)

    def test_marker_attribution_distinguishes_machine_disks_and_linked_nodes(self):
        source = (ROOT / 'docs/CONFIGURATION.md').read_text()
        guide = ' '.join(source.split('### Read active-alert markers\n', 1)[1]
                         .split('\n### ', 1)[0].split())
        model = (ROOT / 'frontend-modern/src/features/alerts/thresholds/thresholdsResourceModel.ts').read_text()
        host = (ROOT / 'frontend-modern/src/features/alerts/thresholds/hooks/useThresholdsHostData.ts').read_text()
        self.assertIn("child.startsWith('disk:') || child.startsWith('disk_temp:')", model)
        self.assertIn("row.type === 'agent' && index.byMachine", model)
        self.assertIn('alertResourceIds: uniqueIds(node.proxmox?.sourceId, node.id)', host)
        self.assertIn('agentAlertResourceId(', host)
        for boundary in ('**Disk %** or **Disk Temp**', 'exact filesystem or device',
                         'does not mean every disk is affected',
                         "agent's CPU, memory and disk alerts", 'not the node',
                         'Check the owning row rather than changing the node'):
            with self.subTest(boundary=boundary):
                self.assertIn(boundary, guide)


if __name__ == '__main__':
    unittest.main()
