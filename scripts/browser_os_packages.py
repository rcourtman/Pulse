#!/usr/bin/env python3
"""Reuse verified browser OS packages across disposable E2E runners.

Since 22 October 2026 some GitHub-hosted runners download Ubuntu packages from
the Azure mirror at tens of kB/s, so the 120 MB Chromium and WebKit
dependency set can exceed the install budget before any test runs
(actions/runner-images#14823 and #15040). The E2E jobs keep the last package
set in an Actions cache instead.

apt reuses a file already in its archive directory after checking only its
size (apt 2.8 ``pkgAcqArchive::QueueNext``), so a restored file is admitted
only when its SHA256 equals the value in the freshly fetched, signed package
index. Package identity comes from the archive filename apt writes, never
from the untrusted file's contents.

  seed CACHE_DIR     refresh the index, then copy verified cached packages
                     into apt's archive directory
  collect CACHE_DIR  replace CACHE_DIR with apt's current archives and set
                     the step output changed=true|false
"""
from __future__ import annotations

import argparse
import hashlib
import os
import re
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path
from urllib.parse import unquote

sys.path.insert(0, str(Path(__file__).resolve().parent))
from write_github_output import append_output  # noqa: E402

ARCHIVES = Path("/var/cache/apt/archives")
PACKAGE_RE = re.compile(r"^[a-z0-9][a-z0-9+.-]+$")
VERSION_RE = re.compile(r"^[0-9A-Za-z.+~:-]+$")
ARCH_RE = re.compile(r"^[a-z0-9-]+$")
QUERY_BATCH = 50

Identity = tuple[str, str, str]


def parse_archive_name(name: str) -> Identity | None:
    """Return (package, version, architecture) for an apt archive filename.

    apt stores ``name_version_arch.deb`` with ``_`` and ``:`` percent-encoded
    inside each part, so the version's epoch appears as ``%3a``.
    """
    if not name.endswith(".deb"):
        return None
    parts = name[: -len(".deb")].split("_")
    if len(parts) != 3:
        return None
    package, version, arch = (unquote(part) for part in parts)
    if not (PACKAGE_RE.fullmatch(package) and VERSION_RE.fullmatch(version) and ARCH_RE.fullmatch(arch)):
        return None
    return package, version, arch


def archive_files(directory: Path) -> dict[str, Path]:
    """Regular, correctly named .deb files directly inside *directory*."""
    if not directory.is_dir():
        return {}
    files = {}
    for path in directory.iterdir():
        if path.is_symlink() or not path.is_file():
            continue
        if parse_archive_name(path.name) is not None:
            files[path.name] = path
    return files


def index_hashes(identities: list[Identity], apt_cache: str) -> dict[Identity, set[str]]:
    """Every SHA256 apt's current index lists for each requested package version.

    Queries name ``package:arch`` without a version: apt-cache then prints
    every record for that architecture, whereas ``=version`` keeps only the
    first, which would drop another repository's hash for the same version.
    The exact package, version, and architecture are matched afterwards.
    """
    wanted = set(identities)
    found: dict[Identity, set[str]] = {}
    queries = sorted({f"{package}:{arch}" for package, _, arch in identities})
    for start in range(0, len(queries), QUERY_BATCH):
        # apt-cache can exit non-zero when a package is unknown; the records
        # it does print are still used.
        output = subprocess.run(
            [apt_cache, "show", *queries[start:start + QUERY_BATCH]],
            capture_output=True, text=True, check=False,
        ).stdout
        for stanza in output.split("\n\n"):
            fields = dict(
                line.split(": ", 1) for line in stanza.splitlines()
                if ": " in line and not line.startswith(" ")
            )
            identity = (fields.get("Package", ""), fields.get("Version", ""), fields.get("Architecture", ""))
            digest = fields.get("SHA256", "")
            if identity in wanted and re.fullmatch(r"[0-9a-f]{64}", digest):
                found.setdefault(identity, set()).add(digest)
    return found


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for block in iter(lambda: handle.read(1 << 20), b""):
            digest.update(block)
    return digest.hexdigest()


def private_copy(source: Path, directory: Path) -> tuple[Path, str]:
    """Copy *source* into a private directory, hashing exactly the bytes copied.

    The copy, not the cache file, is what reaches apt, so replacing the cache
    file after hashing cannot put unverified bytes into the archives.
    """
    copy = directory / source.name
    digest = hashlib.sha256()
    with source.open("rb") as reader, copy.open("xb") as writer:
        for block in iter(lambda: reader.read(1 << 20), b""):
            digest.update(block)
            writer.write(block)
    return copy, digest.hexdigest()


def install_file(source: Path, archives: Path) -> None:
    destination = archives / source.name
    if os.access(archives, os.W_OK):
        shutil.copyfile(source, destination)
        destination.chmod(0o644)
    else:
        subprocess.run(["sudo", "install", "-m", "0644", str(source), str(destination)], check=True)


def seed(cache_dir: Path, archives: Path, apt_cache: str, update: list[str] | None) -> int:
    cached = archive_files(cache_dir)
    if not cached:
        print("No cached browser OS packages to admit")
        return 0
    if update:
        subprocess.run(update, check=True)
    identities = {name: parse_archive_name(name) for name in cached}
    expected = index_hashes(list(identities.values()), apt_cache)
    admitted = rejected = unknown = 0
    with tempfile.TemporaryDirectory(prefix="pulse-browser-os-") as private:
        for name, path in sorted(cached.items()):
            want = expected.get(identities[name])
            if not want:
                unknown += 1
                continue
            copy, got = private_copy(path, Path(private))
            if got not in want:
                rejected += 1
                print(f"Rejected {name}: SHA256 does not match the signed index")
            else:
                install_file(copy, archives)
                admitted += 1
            copy.unlink()
    print(f"Admitted {admitted} cached browser OS packages; {unknown} no longer in the index; {rejected} rejected")
    return 0


def collect(cache_dir: Path, archives: Path) -> int:
    current = archive_files(archives)
    cached = archive_files(cache_dir)
    previous = set(cached)
    # Compare content too: a same-named cached file that seed rejected must
    # be replaced, or every run would reject it and download again.
    changed = bool(current) and (
        set(current) != previous
        or any(sha256(path) != sha256(cached[name]) for name, path in current.items())
    )
    if not current:
        print("::warning::apt kept no downloaded browser OS packages; the cache is left unchanged")
    elif changed:
        # Rebuild in a fresh directory so nothing restored (a symlink, a
        # directory, a stale file) survives or redirects a write.
        fresh = cache_dir.with_name(cache_dir.name + ".new")
        if fresh.is_symlink() or fresh.is_file():
            fresh.unlink()
        elif fresh.exists():
            shutil.rmtree(fresh)
        fresh.mkdir(parents=True)
        for name, path in current.items():
            shutil.copyfile(path, fresh / name)
        if cache_dir.is_symlink() or cache_dir.is_file():
            cache_dir.unlink()
        elif cache_dir.exists():
            shutil.rmtree(cache_dir)
        fresh.rename(cache_dir)
        print(f"Collected {len(current)} browser OS packages ({len(set(current) - previous)} new)")
    else:
        print(f"The {len(current)} browser OS packages match the restored cache")
    output = os.environ.get("GITHUB_OUTPUT")
    if output:
        append_output(Path(output), "changed", "true" if changed else "false")
    return 0


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__.split("\n", 1)[0])
    parser.add_argument("command", choices=("seed", "collect"))
    parser.add_argument("cache_dir", type=Path)
    parser.add_argument("--archives", type=Path, default=ARCHIVES)
    parser.add_argument("--apt-cache", default="apt-cache")
    parser.add_argument("--no-update", action="store_true", help="skip apt-get update (tests)")
    args = parser.parse_args(argv)
    if args.command == "seed":
        update = None if args.no_update else ["sudo", "apt-get", "update"]
        return seed(args.cache_dir, args.archives, args.apt_cache, update)
    return collect(args.cache_dir, args.archives)


if __name__ == "__main__":
    sys.exit(main())
