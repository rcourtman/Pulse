#!/usr/bin/env python3
"""Classify documentation, diagnostic recipes and mirrored Help edits.

Public docs already runs these suites and mirror/link checks. The two diagnostic
recipes have a distinct route: the caller must run both in a required check
before marking application CI inapplicable. Only a nonempty diff exclusively
of those two paths can take that route. The workflow limits both routes to main.
Post-merge Help routing additionally requires a direct main push with only
modifications to existing mirror pairs and rendered Help tests.
"""

import argparse
import re
import subprocess
import sys


COMMIT = re.compile(r"[0-9a-f]{40}")
DIAGNOSTIC_RECIPES = frozenset({
    b"scripts/tests/test_troubleshooting_logs.py",
    b"scripts/tests/test_vm_disk_diagnostics.py",
})
DOC_SUITES = frozenset({
    b"scripts/tests/test_admin_docs.py",
    b"scripts/tests/test_agent_auth_docs.py",
    b"scripts/tests/test_agent_docs.py",
    b"scripts/tests/test_agent_helper_update_docs.py",
    b"scripts/tests/test_agent_log_docs.py",
    b"scripts/tests/test_agent_profile_docs.py",
    b"scripts/tests/test_agent_storage_docs.py",
    b"scripts/tests/test_agent_uninstall_docs.py",
    b"scripts/tests/test_agent_unlink_docs.py",
    b"scripts/tests/test_alerting_outage_help_docs.py",
    b"scripts/tests/test_api_auth_docs.py",
    b"scripts/tests/test_api_secret_body_docs.py",
    b"scripts/tests/test_audit_recovery_docs.py",
    b"scripts/tests/test_authentication_diagnostic_docs.py",
    b"scripts/tests/test_availability_help_docs.py",
    b"scripts/tests/test_bootstrap_import_docs.py",
    b"scripts/tests/test_cloud_migration_docs.py",
    b"scripts/tests/test_configuration_auth_docs.py",
    b"scripts/tests/test_deployment_update_docs.py",
    b"scripts/tests/test_docker_registry_docs.py",
    b"scripts/tests/test_docker_resource_sizing_docs.py",
    b"scripts/tests/test_docker_troubleshooting_docs.py",
    b"scripts/tests/test_docs_mirror.py",
    b"scripts/tests/test_external_agent_docs.py",
    b"scripts/tests/test_failed_upgrade_intake_docs.py",
    b"scripts/tests/test_faq_deployment_docs.py",
    b"scripts/tests/test_guest_disk_reading_docs.py",
    b"scripts/tests/test_guest_network_troubleshooting_docs.py",
    b"scripts/tests/test_http_fixture_docs.py",
    b"scripts/tests/test_icmp_privilege_docs.py",
    b"scripts/tests/test_license_recovery_docs.py",
    b"scripts/tests/test_localized_public_docs.py",
    b"scripts/tests/test_login_troubleshooting_docs.py",
    b"scripts/tests/test_memory_troubleshooting_docs.py",
    b"scripts/tests/test_metric_alert_help_docs.py",
    b"scripts/tests/test_metrics_storage_docs.py",
    b"scripts/tests/test_msp_connection_docs.py",
    b"scripts/tests/test_navigation_help_docs.py",
    b"scripts/tests/test_node_api_docs.py",
    b"scripts/tests/test_notification_troubleshooting_docs.py",
    b"scripts/tests/test_oidc_session_docs.py",
    b"scripts/tests/test_paid_docker_help_docs.py",
    b"scripts/tests/test_password_recovery_docs.py",
    b"scripts/tests/test_pbs_docs.py",
    b"scripts/tests/test_performance_troubleshooting_docs.py",
    b"scripts/tests/test_pmg_docs.py",
    b"scripts/tests/test_production_rollout_docs.py",
    b"scripts/tests/test_public_docs_claims.py",
    b"scripts/tests/test_public_report_containment_docs.py",
    b"scripts/tests/test_pve_backup_troubleshooting_docs.py",
    b"scripts/tests/test_raid_troubleshooting_docs.py",
    b"scripts/tests/test_recovery_api_docs.py",
    b"scripts/tests/test_repo_docs_link_drift.py",
    b"scripts/tests/test_running_version_help_docs.py",
    b"scripts/tests/test_scheduler_health_api_help_docs.py",
    b"scripts/tests/test_scheduler_polling_docs.py",
    b"scripts/tests/test_security_docs.py",
    b"scripts/tests/test_server_https_docs.py",
    b"scripts/tests/test_server_update_containment_docs.py",
    b"scripts/tests/test_setup_access_docs.py",
    b"scripts/tests/test_signed_installer_docs.py",
    b"scripts/tests/test_stale_reading_help_docs.py",
    b"scripts/tests/test_transparency_docs.py",
    b"scripts/tests/test_truenas_docs.py",
    b"scripts/tests/test_unraid_troubleshooting_docs.py",
    b"scripts/tests/test_update_recovery_docs.py",
    b"scripts/tests/test_upgrade_v5_docs.py",
    b"scripts/tests/test_upgrade_v6_docs.py",
    b"scripts/tests/test_watchdog_help_docs.py",
    b"scripts/tests/test_webhook_verification_docs.py",
})
DOCUMENT = re.compile(
    rb"(?:[A-Za-z0-9_.-]+\.md|(?:docs|frontend-modern/public/docs)/[A-Za-z0-9_./-]+\.md)"
)
HELP_GUIDE = re.compile(rb"(docs|frontend-modern/public/docs)/([A-Za-z0-9_-]+\.md)")
HELP_TEST = re.compile(rb"frontend-modern/src/features/docs/__tests__/[A-Za-z0-9_-]+\.test\.ts")
REGULAR_MODIFICATION = re.compile(rb":100644 100644 [a-f0-9]{40} [a-f0-9]{40} M")


def docs_only_paths(paths):
    # NUL-framed Git names are bytes. No display quoting, newline splitting,
    # wildcard suite admission, or rename destination can hide a source edit.
    return bool(paths) and all(
        path in DOC_SUITES or (
            DOCUMENT.fullmatch(path) is not None
            and all(part not in (b"", b".", b"..") for part in path.split(b"/"))
        ) for path in paths
    )


def changed_tree_paths(base, head):
    if not COMMIT.fullmatch(base) or not COMMIT.fullmatch(head):
        return []
    try:
        result = subprocess.run(
            ["git", "diff", "--no-ext-diff", "--no-renames", "--name-only", "-z",
             base, head, "--"],
            capture_output=True, timeout=30, check=False,
        )
    except (OSError, subprocess.TimeoutExpired):
        print("Documentation diff unavailable; retaining application CI.", file=sys.stderr)
        return []
    if result.returncode != 0 or not result.stdout.endswith(b"\0"):
        return []
    return result.stdout[:-1].split(b"\0")


def docs_only_diff(base, head):
    return docs_only_paths(changed_tree_paths(base, head))


def help_push_diff(base, head):
    if not COMMIT.fullmatch(base) or not COMMIT.fullmatch(head):
        return False
    try:
        commit = subprocess.check_output(["git", "cat-file", "commit", head], timeout=30)
        # The push must advance from the exact first parent. cat-file works
        # with the shallow workflow checkout; a stale base is not a shortcut.
        if commit.split(b"\n", 2)[1:2] != [b"parent " + base.encode("ascii")]:
            return False
        raw = subprocess.check_output(
            ["git", "diff", "--raw", "--no-abbrev", "--no-ext-diff", "--no-renames",
             "--ignore-submodules=none", "-z",
             base, head, "--"], timeout=30,
        )
    except (OSError, subprocess.SubprocessError):
        print("Help push diff unavailable; retaining ordinary CI.", file=sys.stderr)
        return False
    records = raw.split(b"\0")
    if records.pop() != b"" or len(records) % 2:
        return False
    canonical, shipped = set(), set()
    has_test = False
    for header, path in zip(records[::2], records[1::2]):
        # Refuse additions, removal, renames, executables and type changes.
        if not REGULAR_MODIFICATION.fullmatch(header):
            return False
        guide = HELP_GUIDE.fullmatch(path)
        if guide:
            root, name = guide.groups()
            blob = header.split()[3]
            (canonical if root == b"docs" else shipped).add((name, blob))
        elif HELP_TEST.fullmatch(path):
            has_test = True
        else:
            return False
    return has_test and bool(canonical) and canonical == shipped


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--base", required=True)
    parser.add_argument("--head", required=True)
    mode = parser.add_mutually_exclusive_group()
    mode.add_argument("--route", action="store_true", help="also classify isolated diagnostic recipe edits")
    mode.add_argument("--help-push", action="store_true", help="only for the main push workflow")
    args = parser.parse_args()
    if args.help_push:
        print("true" if help_push_diff(args.base, args.head) else "false")
        return
    if not args.route:
        print("true" if docs_only_diff(args.base, args.head) else "false")
        return
    paths = changed_tree_paths(args.base, args.head)
    if paths and all(path in DIAGNOSTIC_RECIPES for path in paths):
        print("diagnostic-recipes")
    else:
        print("docs" if docs_only_paths(paths) else "runtime")


if __name__ == "__main__":
    main()
