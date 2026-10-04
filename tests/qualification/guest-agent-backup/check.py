#!/usr/bin/env python3
"""Check a bounded, redacted native backup record. No network or guest commands.

Supplied provenance is evidence for independent review, not authority or an
independently verified artifact. A successful check is not release qualification.
"""

import argparse
import hashlib
import json
from pathlib import Path
import re
import sys

MAX_BYTES = 2 * 1024 * 1024
ID = re.compile(r"[a-z0-9][a-z0-9-]{0,31}\Z")
HEX40 = re.compile(r"[0-9a-f]{40}\Z")
HEX64 = re.compile(r"[0-9a-f]{64}\Z")
METHODS = {"get-fsinfo", "file-read", "get-osinfo", "info", "network-get-interfaces"}


class InvalidRecord(Exception):
    pass


def require(ok, message):
    if not ok:
        raise InvalidRecord(message)


def shape(value, keys, label):
    require(type(value) is dict and set(value) == set(keys.split()), label + " shape")


def integer(value):
    return type(value) is int and 0 <= value < 2**53


def digest(value, pattern=HEX64):
    return type(value) is str and pattern.fullmatch(value) is not None


def unique_object(pairs):
    result = {}
    for key, value in pairs:
        require(key not in result, "duplicate JSON field")
        result[key] = value
    return result


def decode(raw):
    try:
        return json.loads(raw, object_pairs_hook=unique_object)
    except (ValueError, UnicodeError, RecursionError):
        raise InvalidRecord("invalid JSON") from None


def read_bounded(path):
    require(not path.is_symlink() and path.is_file(), "regular evidence file required")
    with path.open("rb") as stream:
        raw = stream.read(MAX_BYTES + 1)
    require(0 < len(raw) <= MAX_BYTES, "evidence size bound")
    return raw


def witness_events(root, entry, run_id, skew):
    shape(entry, "filesystem_id file sha256 exit_code", "witness")
    require(type(entry["filesystem_id"]) is str and ID.fullmatch(entry["filesystem_id"]), "filesystem ID")
    name = entry["file"]
    require(type(name) is str and re.fullmatch(r"[a-zA-Z0-9][a-zA-Z0-9_.-]{0,95}\.jsonl", name), "witness filename")
    require(digest(entry["sha256"]), "witness digest")
    require(type(entry["exit_code"]) is int and entry["exit_code"] == 0, "witness did not exit successfully")
    raw = read_bounded(root / name)
    require(hashlib.sha256(raw).hexdigest() == entry["sha256"], "witness content mismatch")
    require(raw.endswith(b"\n"), "incomplete witness line")
    lines = raw.splitlines()
    require(4 <= len(lines) <= 2000 and all(0 < len(line) <= 2048 for line in lines), "witness event bound")
    events = [decode(line) for line in lines]
    previous_ms, previous_elapsed, sequence, previous_write_ms = -1, -1, 0, -1
    for i, event in enumerate(events):
        kind = event.get("kind") if type(event) is dict else None
        require(type(kind) is str, "witness event kind")
        extra = {"start": "", "heartbeat": "", "write": " sequence started_unix_ms sha256", "stop": " result"}.get(kind)
        require(extra is not None, "witness event kind")
        shape(event, "schema_version kind run_id filesystem_id unix_ms elapsed_ms" + extra, "witness event")
        require(type(event["schema_version"]) is int and event["schema_version"] == 1, "witness schema")
        require(event["run_id"] == run_id and event["filesystem_id"] == entry["filesystem_id"], "witness identity mismatch")
        at, elapsed = event["unix_ms"], event["elapsed_ms"]
        require(integer(at) and integer(elapsed) and at >= previous_ms and elapsed >= previous_elapsed, "witness time order")
        require(elapsed <= 601000, "witness duration bound")
        if i:
            drift = (at - events[0]["unix_ms"]) - (elapsed - events[0]["elapsed_ms"])
            require(abs(drift) <= skew, "witness clock discontinuity")
        require((kind == "start") == (i == 0) and (kind == "stop") == (i == len(events) - 1), "witness lifecycle")
        if kind == "write":
            sequence += 1
            require(type(event["sequence"]) is int and event["sequence"] == sequence, "witness sequence")
            require(integer(event["started_unix_ms"]) and
                    max(previous_write_ms, events[0]["unix_ms"]) <= event["started_unix_ms"] <= at, "witness write time")
            payload = f"Pulse disposable backup witness\n{run_id}\n{entry['filesystem_id']}\n{sequence}\n".encode()
            payload += b"x" * (512 - len(payload))
            require(digest(event["sha256"]) and event["sha256"] == hashlib.sha256(payload).hexdigest(), "witness readback digest")
            previous_write_ms = at
        if kind == "stop":
            require(event["result"] == "complete", "witness incomplete or failed")
        previous_ms, previous_elapsed = at, elapsed
    return events


def check_record(record, root, expected_source):
    require(digest(expected_source, HEX40), "expected source SHA")
    shape(record, "schema_version run_id evidence_class identity preflight window backup coverage commands snapshots witnesses", "record")
    require(type(record["schema_version"]) is int and record["schema_version"] == 1, "record schema")
    require(type(record["run_id"]) is str and ID.fullmatch(record["run_id"]), "run ID")
    require(record["evidence_class"] == "native-disposable", "not a native disposable record")
    identity = record["identity"]
    shape(identity, "pulse_source pulse_binary_sha256 pve_packages_sha256 qga_binary_sha256 haos_image_sha256 provenance_receipt_sha256", "identity")
    require(identity["pulse_source"] == expected_source, "installed source mismatch")
    require(all(digest(identity[k]) for k in identity if k != "pulse_source"), "input identity digest")
    preflight = record["preflight"]
    shape(preflight, "pve_version guest_type audit file_read tls_verified qga_enabled freeze_enabled single_pulse_process filesystem_map_sha256 filesystem_ids", "preflight")
    require(preflight["pve_version"] == "9.2.21" and preflight["guest_type"] == "HAOS", "reported platform mismatch")
    require(all(preflight[k] is True for k in ("audit", "file_read", "tls_verified", "qga_enabled", "freeze_enabled", "single_pulse_process")), "preflight incomplete")
    require(digest(preflight["filesystem_map_sha256"]), "filesystem crosswalk digest")
    expected_filesystems = preflight["filesystem_ids"]
    require(type(expected_filesystems) is list and 1 <= len(expected_filesystems) <= 16 and
            all(type(v) is str and ID.fullmatch(v) for v in expected_filesystems) and
            len(set(expected_filesystems)) == len(expected_filesystems), "covered filesystem IDs")
    window = record["window"]
    shape(window, "start_ms end_ms clock_error_ms", "window")
    require(all(integer(v) for v in window.values()), "observation bounds")
    start, end, skew = window["start_ms"], window["end_ms"], window["clock_error_ms"]
    require(3000 <= end - start <= 600000 and skew <= 1000, "observation duration or clock bound")
    backup = record["backup"]
    shape(backup, "start_ms lock_start_ms freeze_ms thaw_request_ms lock_end_ms completed_ms status trace_sha256", "backup")
    times = [backup[k] for k in ("start_ms", "lock_start_ms", "freeze_ms", "thaw_request_ms", "lock_end_ms", "completed_ms")]
    require(all(integer(v) for v in times) and times == sorted(times) and start < times[0] < times[-1] < end, "backup time order")
    require(backup["lock_start_ms"] < backup["lock_end_ms"], "backup lock not observed")
    require(backup["status"] == "OK" and digest(backup["trace_sha256"]), "backup did not complete normally")
    coverage = record["coverage"]
    shape(coverage, "start_ms end_ms dropped_events trace_sha256", "coverage")
    require(all(integer(coverage[k]) for k in ("start_ms", "end_ms", "dropped_events")) and
            coverage["start_ms"] <= start and coverage["end_ms"] >= end and coverage["dropped_events"] == 0 and
            digest(coverage["trace_sha256"]), "native command coverage incomplete")

    commands = record["commands"]
    require(type(commands) is list and 1 <= len(commands) <= 512, "native command bound")
    ids, intervals = set(), []
    overlap = False
    for command in commands:
        shape(command, "id method start_ms end_ms result", "command")
        require(type(command["id"]) is str and ID.fullmatch(command["id"]) and command["id"] not in ids, "duplicate or invalid command ID")
        ids.add(command["id"])
        require(type(command["method"]) is str and command["method"] in METHODS and command["result"] == "ok", "native command adverse or unsupported")
        began, finished = command["start_ms"], command["end_ms"]
        require(integer(began) and integer(finished) and start <= began < finished <= end, "command time order")
        require(not backup["lock_start_ms"] - skew <= began <= backup["lock_end_ms"] + skew, "command dispatched during lock or uncertainty")
        overlap |= began < backup["lock_start_ms"] - skew and finished > backup["lock_start_ms"] + skew
        intervals.append((began, finished))
    require(overlap, "no observed in-flight overlap")
    intervals.sort()
    require(all(a[1] <= b[0] for a, b in zip(intervals, intervals[1:])), "concurrent guest commands")

    snapshots = record["snapshots"]
    require(type(snapshots) is list and 4 <= len(snapshots) <= 64, "monitoring snapshot bound")
    phases = [s.get("phase") if type(s) is dict else None for s in snapshots]
    require(phases == ["before"] + ["locked"] * (len(snapshots) - 2) + ["resumed"], "monitoring phases")
    for snapshot in snapshots:
        shape(snapshot, "phase at_ms guest_identity_sha256 guest_status metadata_sha256 memory_sha256 disk_sha256 memory_source disk_reason disk_history_latest_ms memory_history_latest_ms cpu_latest_ms", "snapshot")
        require(integer(snapshot["at_ms"]), "snapshot time")
        require(all(digest(snapshot[k]) for k in ("guest_identity_sha256", "metadata_sha256", "memory_sha256", "disk_sha256")), "snapshot digest")
        require(all(integer(snapshot[k]) and start <= snapshot[k] <= snapshot["at_ms"] <= end for k in
                    ("at_ms", "disk_history_latest_ms", "memory_history_latest_ms", "cpu_latest_ms")), "snapshot times")
    require(all(a["at_ms"] < b["at_ms"] for a, b in zip(snapshots, snapshots[1:])), "snapshot time order")
    before, resumed = snapshots[0], snapshots[-1]
    require(before["at_ms"] < backup["start_ms"] - skew and before["guest_status"] == "available" and
            before["memory_source"] == "guest-agent-meminfo" and before["disk_reason"] == "", "baseline guest truth missing")
    stable = ("guest_identity_sha256", "metadata_sha256", "memory_sha256", "disk_sha256", "disk_history_latest_ms", "memory_history_latest_ms")
    for locked in snapshots[1:-1]:
        require(backup["lock_start_ms"] + skew < locked["at_ms"] < backup["lock_end_ms"] - skew,
                "locked snapshot outside lock")
        require(locked["guest_status"] == "deferred" and locked["memory_source"] == "previous-snapshot" and
                locked["disk_reason"] == "prev-vm-locked", "deferral not visible")
        require(all(locked[k] == before[k] for k in stable), "retained evidence renewed or changed during lock")
        require(locked["cpu_latest_ms"] > before["cpu_latest_ms"], "non-QGA monitoring did not progress")
    require(resumed["guest_identity_sha256"] == before["guest_identity_sha256"] and resumed["guest_status"] == "available" and
            resumed["memory_source"] == "guest-agent-meminfo" and resumed["disk_reason"] == "", "guest did not resume")
    require(all(resumed[k] > backup["completed_ms"] + skew for k in ("disk_history_latest_ms", "memory_history_latest_ms", "cpu_latest_ms")),
            "fresh post-backup History absent")
    # A fresh timestamp is not evidence of a new guest read. The complete
    # native trace must include both kinds of reads supporting this resumed
    # snapshot, not just the pre-lock in-flight command or an old cached result.
    # Allow the recorded clock error when comparing completion with readback;
    # do not accept a start inside the post-task uncertainty interval.
    for method in ("get-fsinfo", "file-read"):
        require(any(command["method"] == method and
                    command["start_ms"] > backup["completed_ms"] + skew and
                    command["end_ms"] <= resumed["at_ms"] + skew for command in commands),
                "post-task " + method + " read supporting resumption missing")

    witnesses = record["witnesses"]
    require(type(witnesses) is list and 1 <= len(witnesses) <= 16, "filesystem witness bound")
    filesystems = set()
    for witness in witnesses:
        events = witness_events(root, witness, record["run_id"], skew)
        require(witness["filesystem_id"] not in filesystems, "duplicate filesystem witness")
        filesystems.add(witness["filesystem_id"])
        require(events[0]["unix_ms"] < backup["start_ms"] - skew and end - skew <= events[-1]["unix_ms"] <= end + skew,
                "witness did not cover the observation window")
        writes = [e for e in events if e["kind"] == "write"]
        require(sum(e["unix_ms"] < backup["start_ms"] - skew for e in writes) >= 2, "baseline filesystem writes missing")
        require(sum(backup["completed_ms"] + skew < e["started_unix_ms"] <= e["unix_ms"] <= end + skew for e in writes) >= 2,
                "independent post-task fsync/readback missing")
        require(any(e["kind"] == "heartbeat" and backup["freeze_ms"] + skew < e["unix_ms"] < backup["thaw_request_ms"] - skew for e in events),
                "guest liveness during freeze missing")
    require(filesystems == set(expected_filesystems), "covered filesystem witness missing")
    return {"record_checks_passed": True, "native_acceptance_complete": False,
            "source": expected_source, "filesystem_witnesses": len(filesystems),
            "limits": "Review native provenance, covered-filesystem completeness and raw traces independently. This record does not reproduce the reported cause, certify all guests, establish delivery or qualify a release."}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--record", type=Path, required=True)
    parser.add_argument("--expected-source", required=True)
    args = parser.parse_args()
    try:
        result = check_record(decode(read_bounded(args.record)), args.record.parent, args.expected_source)
    except (InvalidRecord, OSError, TypeError, ValueError, RecursionError) as error:
        # The record may contain private or malformed input. Do not echo it or
        # exception paths. Detailed receipts remain with the native owner.
        failure = str(error) if isinstance(error, InvalidRecord) else "evidence read or schema failure"
        print(json.dumps({"record_checks_passed": False, "native_acceptance_complete": False, "failure": failure}), file=sys.stderr)
        return 1
    print(json.dumps(result, sort_keys=True))
    return 0


if __name__ == "__main__":
    sys.exit(main())
