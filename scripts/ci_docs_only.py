#!/usr/bin/env python3
"""Keep isolated documentation safety edits on the complete Public docs check.

These exact suites already run in Public docs, along with mirror/link checks.
They do not ship in Pulse. New suites and admission/workflow controls retain
application CI until their coverage is reviewed. Mixed, empty and unavailable
whole-tree diffs cannot select this exception. The workflow limits it to main,
the only branch with both push and pull-request Public docs coverage.
"""

import argparse
import re
import subprocess
import sys


COMMIT = re.compile(r"[0-9a-f]{40}")
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
    b"scripts/tests/test_api_auth_docs.py",
    b"scripts/tests/test_api_secret_body_docs.py",
    b"scripts/tests/test_audit_recovery_docs.py",
    b"scripts/tests/test_authentication_diagnostic_docs.py",
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
    b"scripts/tests/test_scheduler_polling_docs.py",
    b"scripts/tests/test_security_docs.py",
    b"scripts/tests/test_server_update_containment_docs.py",
    b"scripts/tests/test_setup_access_docs.py",
    b"scripts/tests/test_signed_installer_docs.py",
    b"scripts/tests/test_transparency_docs.py",
    b"scripts/tests/test_truenas_docs.py",
    b"scripts/tests/test_unraid_troubleshooting_docs.py",
    b"scripts/tests/test_update_recovery_docs.py",
    b"scripts/tests/test_upgrade_v5_docs.py",
    b"scripts/tests/test_upgrade_v6_docs.py",
    b"scripts/tests/test_webhook_verification_docs.py",
})
DOCUMENT = re.compile(
    rb"(?:[A-Za-z0-9_.-]+\.md|(?:docs|frontend-modern/public/docs)/[A-Za-z0-9_./-]+\.md)"
)


def docs_only_paths(paths):
    # NUL-framed Git names are bytes. No display quoting, newline splitting,
    # wildcard suite admission, or rename destination can hide a source edit.
    return bool(paths) and all(
        path in DOC_SUITES or (
            DOCUMENT.fullmatch(path) is not None
            and all(part not in (b"", b".", b"..") for part in path.split(b"/"))
        ) for path in paths
    )


def docs_only_diff(base, head):
    if not COMMIT.fullmatch(base) or not COMMIT.fullmatch(head):
        return False
    try:
        result = subprocess.run(
            ["git", "diff", "--no-ext-diff", "--no-renames", "--name-only", "-z",
             base, head, "--"],
            capture_output=True, timeout=30, check=False,
        )
    except (OSError, subprocess.TimeoutExpired):
        print("Documentation diff unavailable; retaining application CI.", file=sys.stderr)
        return False
    if result.returncode != 0 or not result.stdout.endswith(b"\0"):
        return False
    return docs_only_paths(result.stdout[:-1].split(b"\0"))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--base", required=True)
    parser.add_argument("--head", required=True)
    args = parser.parse_args()
    print("true" if docs_only_diff(args.base, args.head) else "false")


if __name__ == "__main__":
    main()
