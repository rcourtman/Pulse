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


if __name__ == '__main__':
    unittest.main()
