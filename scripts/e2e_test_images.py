#!/usr/bin/env python3
"""Build once, then admit the same run/source-bound E2E images in every job.

This is a same-run artifact, not a registry mirror or a cross-run image cache.
Docker failures are terminal: no login, alternate registry, pull retry or rebuild
fallback is performed. Only the existing secret-free E2E stack is distributed.
"""

import argparse
import gzip
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys


ROOT = Path(__file__).resolve().parents[1]
COMPOSE = ROOT / "tests/integration/docker-compose.test.yml"
ARCHIVE = "images.tar.gz"
MANIFEST = "manifest.json"
IMAGES = ("pulse:test", "pulse-mock-github:test", "pulse-e2e-seed:test")
REVISION_LABEL = "org.opencontainers.image.revision"
SHA256 = r"[0-9a-f]{64}"
MAX_ARCHIVE_BYTES = 1024 * 1024 * 1024


def run(*args, capture=False):
    return subprocess.run(
        args, cwd=ROOT, check=True, text=True,
        stdout=subprocess.PIPE if capture else None,
    ).stdout


def digest(path):
    with path.open("rb") as stream:
        return hashlib.file_digest(stream, "sha256").hexdigest()


def seed_image():
    # Follow the committed compose pin, never an environment/registry override.
    block = COMPOSE.read_text().split("  seed-bootstrap-token:\n", 1)[1].split("\n  mock-github:", 1)[0]
    match = re.search(
        r"(?m)^    image: \$\{PULSE_E2E_SEED_IMAGE:-(alpine:[0-9.]+@sha256:" + SHA256 + r")\}$",
        block,
    )
    if not match:
        raise ValueError("E2E seed image must retain its exact committed Alpine pin")
    return match[1]


def binding():
    source = os.environ["GITHUB_SHA"]
    run_id = os.environ["GITHUB_RUN_ID"]
    attempt = os.environ["GITHUB_RUN_ATTEMPT"]
    if not re.fullmatch(r"[0-9a-f]{40}", source):
        raise ValueError("Invalid E2E checkout identity")
    if any(not re.fullmatch(r"[1-9][0-9]*", value) for value in (run_id, attempt)):
        raise ValueError("Invalid E2E run identity")
    if run("git", "rev-parse", "HEAD", capture=True).strip() != source:
        raise ValueError("E2E checkout does not match this workflow source")
    return {"source_sha": source, "run_id": run_id, "run_attempt": attempt}


def inspect_images(source):
    ids = {}
    for image in IMAGES:
        record = json.loads(run("docker", "image", "inspect", "--format", "{{json .}}", image, capture=True))
        if not re.fullmatch("sha256:" + SHA256, record["Id"]):
            raise ValueError("Invalid E2E image identity")
        if record["Os"] != "linux" or record["Architecture"] != "amd64":
            raise ValueError("E2E images must target Linux amd64")
        if image != "pulse-e2e-seed:test":
            labels = record["Config"].get("Labels") or {}
            if labels.get(REVISION_LABEL) != source:
                raise ValueError("E2E image revision does not match the checkout")
        ids[image] = record["Id"]
    return ids


def build(directory, identity):
    directory.mkdir(parents=True, exist_ok=False)
    label = f"{REVISION_LABEL}={identity['source_sha']}"
    # Keep the e2e_runtime target and empty release tags: both mock and
    # non-mock registration jobs already used this exact build contract.
    run("docker", "build", "--platform=linux/amd64", "--label", label,
        "-t", IMAGES[0], "--target", "e2e_runtime", "--build-arg", "GO_BUILD_TAGS=", ".")
    run("docker", "build", "--platform=linux/amd64", "--label", label,
        "-t", IMAGES[1], "./tests/integration/mock-github-server")
    pinned_seed = seed_image()
    run("docker", "image", "pull", "--platform=linux/amd64", pinned_seed)
    run("docker", "image", "tag", pinned_seed, IMAGES[2])
    ids = inspect_images(identity["source_sha"])
    raw = directory / "images.tar"
    run("docker", "image", "save", "--output", str(raw), *IMAGES)
    archive = directory / ARCHIVE
    with raw.open("rb") as incoming, archive.open("xb") as outgoing:
        with gzip.GzipFile(filename="", fileobj=outgoing, mode="wb", compresslevel=1, mtime=0) as compressed:
            shutil.copyfileobj(incoming, compressed, length=1024 * 1024)
    raw.unlink()
    size = archive.stat().st_size
    if not 0 < size <= MAX_ARCHIVE_BYTES:
        raise ValueError("E2E image bundle exceeds the transfer budget")
    manifest = {
        "schema_version": 1, **identity, "seed_image": pinned_seed, "images": ids,
        "archive_sha256": digest(archive), "archive_bytes": size,
    }
    path = directory / MANIFEST
    with path.open("x") as stream:
        json.dump(manifest, stream, sort_keys=True)
        stream.write("\n")
    # The job output is the trust anchor, not a checksum supplied beside bytes.
    with Path(os.environ["GITHUB_OUTPUT"]).open("a") as stream:
        stream.write(f"manifest-sha256={digest(path)}\n")
    print("Built one source-bound E2E bundle (two builds and the pinned seed image)")


def load(directory, identity):
    expected = os.environ["PULSE_E2E_MANIFEST_SHA256"]
    if not re.fullmatch(SHA256, expected):
        raise ValueError("Missing or invalid producer manifest identity")
    manifest_path = directory / MANIFEST
    archive = directory / ARCHIVE
    for path in (manifest_path, archive):
        if path.is_symlink() or not path.is_file():
            raise ValueError("E2E bundle files must be regular files")
    if manifest_path.stat().st_size > 16384 or digest(manifest_path) != expected:
        raise ValueError("E2E manifest differs from the producer job output")
    manifest = json.loads(manifest_path.read_text())
    keys = {"schema_version", *identity, "seed_image", "images", "archive_sha256", "archive_bytes"}
    if not isinstance(manifest, dict) or set(manifest) != keys or type(manifest["schema_version"]) is not int or manifest["schema_version"] != 1:
        raise ValueError("Unsupported E2E manifest")
    if any(manifest[key] != value for key, value in identity.items()):
        raise ValueError("E2E bundle belongs to another source, run or attempt")
    if manifest["seed_image"] != seed_image():
        raise ValueError("E2E seed image differs from the committed compose pin")
    ids = manifest["images"]
    if not isinstance(ids, dict) or set(ids) != set(IMAGES):
        raise ValueError("E2E bundle must contain the three fixed image identities")
    if any(not isinstance(value, str) or not re.fullmatch("sha256:" + SHA256, value) for value in ids.values()):
        raise ValueError("Invalid producer E2E image identities")
    size = archive.stat().st_size
    if type(manifest["archive_bytes"]) is not int or not 0 < size <= MAX_ARCHIVE_BYTES or size != manifest["archive_bytes"]:
        raise ValueError("E2E image archive size does not match the producer")
    if not isinstance(manifest["archive_sha256"], str) or not re.fullmatch(SHA256, manifest["archive_sha256"]):
        raise ValueError("Invalid producer archive identity")
    if digest(archive) != manifest["archive_sha256"]:
        raise ValueError("E2E image archive differs from the producer")
    # No load, start, pull or build occurs before all admission checks pass.
    run("docker", "image", "load", "--input", str(archive))
    if inspect_images(identity["source_sha"]) != ids:
        raise ValueError("Loaded E2E image identities differ from the producer")
    print("Admitted the producer's exact E2E images for this source/run/attempt")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("operation", choices=("build", "load"))
    parser.add_argument("directory", type=Path)
    args = parser.parse_args()
    try:
        identity = binding()
        (build if args.operation == "build" else load)(args.directory, identity)
    except (ValueError, KeyError, IndexError, OSError, subprocess.CalledProcessError) as error:
        print(f"E2E image admission failed: {error}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
