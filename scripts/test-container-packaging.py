#!/usr/bin/env python3
"""Same-payload Docker packaging regression. Run only on disposable CI Docker.

No publication, registry credentials or release qualification. The immutable
published fixture is deliberately independent of the proposed application code.
"""
import argparse
import gzip
import hashlib
import json
import subprocess
import tarfile
import tempfile
import time
import zlib
from pathlib import Path


OWNED_PREFIXES = ("opt/pulse/",)
OWNED_FILES = {"app/pulse", "app/VERSION", "usr/local/bin/pulse-agent", "opt/pulse"}


def owned(name):
    return name in OWNED_FILES or name.startswith(OWNED_PREFIXES)


def digest(stream):
    h = hashlib.sha256()
    for chunk in iter(lambda: stream.read(1024 * 1024), b""):
        h.update(chunk)
    return h.hexdigest()


def inventory(stream, totals=None):
    """Never extract image paths or follow links on the runner."""
    records = {}
    regular_bytes = 0
    with tarfile.open(fileobj=stream, mode="r|*") as archive:
        for member in archive:
            if member.isfile():
                regular_bytes += member.size
            name = member.name.removeprefix("./").rstrip("/")
            if not owned(name):
                continue
            if name in records:
                raise ValueError(f"duplicate exported path: {name}")
            record = dict(mode=member.mode, uid=member.uid, gid=member.gid,
                          type=member.type.decode("ascii"), size=member.size,
                          link=member.linkname)
            if member.isfile():
                record["sha256"] = digest(archive.extractfile(member))
            records[name] = record
    if totals is not None:
        totals["merged_rootfs_regular_file_bytes"] = regular_bytes
    return records


def layers(stream):
    """Docker-save uncompressed bytes and explicit *reencoded* gzip sizes.

    Reencoding each layer is comparable, not the registry's original compression
    or a multi-platform index size. Neither number includes writable volumes.
    """
    result = []
    # Classic Docker archives use <id>/layer.tar; containerd-backed Docker
    # uses blobs/sha256/<digest>. Both name the retained layers in manifest.json.
    # Spool opaque blobs under generated names, never archive-selected paths.
    with tempfile.TemporaryDirectory(prefix="pulse-image-blobs-") as directory:
        blobs = {}
        with tarfile.open(fileobj=stream, mode="r|*") as outer:
            for index, entry in enumerate(outer):
                if not entry.isfile():
                    continue
                if entry.name in blobs:
                    raise ValueError("duplicate Docker-save blob")
                target = Path(directory)/str(index)
                with target.open("wb") as destination:
                    source = outer.extractfile(entry)
                    for chunk in iter(lambda: source.read(1024 * 1024), b""):
                        destination.write(chunk)
                blobs[entry.name] = target
        manifest_file = blobs.get("manifest.json")
        if manifest_file is None or manifest_file.stat().st_size > 1024 * 1024:
            raise ValueError("missing or oversized Docker-save manifest")
        manifest = json.loads(manifest_file.read_bytes())
        if (not isinstance(manifest, list) or len(manifest) != 1 or
                not isinstance(manifest[0], dict) or
                not isinstance(manifest[0].get("Layers"), list) or not manifest[0]["Layers"]):
            raise ValueError("expected exactly one image with filesystem layers")
        names = manifest[0]["Layers"]
        if not all(isinstance(n, str) for n in names) or len(set(names)) != len(names):
            raise ValueError("invalid or duplicate Docker-save layer identity")
        for name in names:
            if name not in blobs:
                raise ValueError("manifest layer blob is missing")
            path = blobs[name]
            compressor = zlib.compressobj(level=6, wbits=31)
            compressed = uncompressed = 0
            with path.open("rb") as raw:
                magic = raw.read(2)
                raw.seek(0)
                decoded = gzip.GzipFile(fileobj=raw) if magic == b"\x1f\x8b" else raw
                for chunk in iter(lambda: decoded.read(1024 * 1024), b""):
                    uncompressed += len(chunk)
                    compressed += len(compressor.compress(chunk))
                compressed += len(compressor.flush())
            with path.open("rb") as raw:
                files = inventory(raw)
            result.append(dict(archive_blob_bytes=path.stat().st_size,
                               uncompressed_tar_bytes=uncompressed,
                               reencoded_gzip_bytes=compressed, files=files))
    return result


def no_payload_copyups(layer_records):
    seen = set()
    for layer in layer_records:
        for name, record in layer["files"].items():
            if record["type"] not in ("0", "\0"):
                continue
            if name in seen:
                raise ValueError(f"payload stored in more than one layer: {name}")
            seen.add(name)
    if "app/pulse" not in seen or not any(n.startswith("opt/pulse/bin/") for n in seen):
        raise ValueError("runtime payload missing from layer inventory")


def compare(before, after):
    if before != after:
        changed = sorted(n for n in before.keys() | after.keys() if before.get(n) != after.get(n))
        raise ValueError(f"packaging changed final payload bytes/metadata/links: {changed}")


def command(*args):
    return subprocess.check_output(args, text=True).strip()


def stream_command(args, reader):
    # Always drain/observe the exact process, including parser failures.
    with subprocess.Popen(args, stdout=subprocess.PIPE) as process:
        try:
            result = reader(process.stdout)
        finally:
            for _ in iter(lambda: process.stdout.read(1024 * 1024), b""):
                pass
            status = process.wait()
        if status:
            raise subprocess.CalledProcessError(status, args)
    return result


# The container has no network other than its own loopback. Every download must
# come from its bundled payload, not GitHub fallback. Check actual headers/body
# through the running server, not a substitute fixture web page.
DOWNLOAD_CHECK = r'''
import base64, hashlib, json, pathlib, urllib.request
root = pathlib.Path('/opt/pulse')
requests = [('/install.sh', root/'scripts/install.sh'),
            ('/install.ps1', root/'scripts/install.ps1')]
linux = ['amd64', 'arm64', 'armv7', 'armv6', '386']
for family, arches in [('pulse-agent', ['linux-'+a for a in linux] +
                       ['darwin-amd64', 'darwin-arm64', 'freebsd-amd64',
                        'freebsd-arm64', 'windows-amd64', 'windows-arm64', 'windows-386']),
                       ('pulse-agent-helper', ['linux-'+a for a in linux]),
                       ('pulse-agent-runner', ['linux-'+a for a in linux])]:
    for arch in arches:
        name = family+'-'+arch+('.exe' if arch.startswith('windows-') else '')
        requests.append(('/download/'+family+'?arch='+arch, root/'bin'/name))
receipts = []
for route, path in requests:
    assert path.stat().st_mode & 0o111, str(path)
    for suffix in ['.sig', '.sshsig']:
        assert not (pathlib.Path(str(path)+suffix).stat().st_mode & 0o111), str(path)+suffix
    with urllib.request.urlopen('http://127.0.0.1:7655'+route, timeout=30) as response:
        h = hashlib.sha256()
        for chunk in iter(lambda: response.read(1024*1024), b''): h.update(chunk)
        with path.open('rb') as f: expected = hashlib.file_digest(f, 'sha256').hexdigest()
        assert h.hexdigest() == expected, route
        if route.startswith('/download/'):
            assert response.headers['X-Checksum-Sha256'] == expected, route
        assert response.headers['X-Signature-Ed25519'] == pathlib.Path(str(path)+'.sig').read_text().strip(), route
        assert base64.b64decode(response.headers['X-Signature-SSHSIG']) == pathlib.Path(str(path)+'.sshsig').read_bytes(), route
        receipts.append({'route': route, 'sha256': expected, 'signature_headers': True})
print(json.dumps(receipts))
'''


def runtime_checks(image, arch):
    receipts = []
    for uid in (1000, 2345):
        container = command("docker", "run", "-d", "--network", "none", "-e", f"PUID={uid}",
                            "-e", f"PGID={uid}", image)
        try:
            deadline = time.monotonic() + 90
            while True:
                status = command("docker", "inspect", "--format", "{{.State.Health.Status}}", container)
                if status == "healthy":
                    break
                if status == "unhealthy" or time.monotonic() > deadline:
                    raise RuntimeError(f"container health did not recover: {status}")
                time.sleep(2)
            actual = command("docker", "exec", container, "sh", "-ec",
                             "awk '/^Uid:/{print $2}' /proc/1/status; stat -c '%u:%g' /data")
            if actual != f"{uid}\n{uid}:{uid}":
                raise ValueError(f"wrong running UID or data owner: {actual}")
            download = json.loads(command("docker", "exec", container, "python3", "-c", DOWNLOAD_CHECK))
            receipts.append(dict(uid=uid, health=status, downloads=download))
        finally:
            subprocess.run(["docker", "rm", "-fv", container], check=True)
    version = command("docker", "run", "--rm", "--network", "none", "--entrypoint",
                      "/usr/local/bin/pulse-agent", image, "--version")
    if "6.6.0" not in version:
        raise ValueError("native agent entrypoint did not execute the fixture version")
    return dict(platform=f"linux/{arch}", native_agent_version=version, server=receipts)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--base", required=True)
    parser.add_argument("--payload", type=Path, required=True)
    parser.add_argument("--arch", choices=("amd64", "arm64"), required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    if len(args.base) != 40 or any(c not in "0123456789abcdef" for c in args.base):
        parser.error("base must be an exact commit")
    head = command("git", "rev-parse", "HEAD")
    baseline = subprocess.check_output(["git", "show", f"{args.base}:Dockerfile"])
    records = dict(source_sha=head, base_sha=args.base, platform=f"linux/{args.arch}",
                   fixture_release="v6.6.0", fixture_release_id=423296089, images={},
                   dockerfiles_sha256=dict(base=hashlib.sha256(baseline).hexdigest(),
                        candidate=hashlib.sha256(Path("Dockerfile").read_bytes()).hexdigest()),
                   size_basis="Per-platform image/layer and fresh merged-rootfs bytes; gzip is reencoded, not original registry transfer. Writable data/volumes excluded.")
    tags = []
    args.output.parent.mkdir(parents=True, exist_ok=True)
    try:
        with tempfile.TemporaryDirectory(prefix="pulse-packaging-") as directory:
            base_file = Path(directory)/"Dockerfile.base"
            base_file.write_bytes(baseline)
            for label, dockerfile in (("base", base_file), ("candidate", Path("Dockerfile"))):
                tag = f"pulse-packaging-{label}:{args.arch}"
                tags.append(tag)
                subprocess.run(["docker", "buildx", "build", "--file", str(dockerfile),
                                "--target", "runtime_prebuilt", "--platform", f"linux/{args.arch}",
                                "--build-context", f"release_payload={args.payload.resolve()}",
                                "--load", "--tag", tag, "."], check=True)
                info = json.loads(command("docker", "image", "inspect", tag))[0]
                image_layers = stream_command(["docker", "image", "save", tag], layers)
                container = command("docker", "create", "--network", "none", tag)
                rootfs_totals = {}
                try:
                    rootfs = stream_command(["docker", "export", container],
                                            lambda stream: inventory(stream, rootfs_totals))
                finally:
                    subprocess.run(["docker", "rm", "-v", container], check=True)
                records["images"][label] = dict(image_id=info["Id"], config=info["Config"],
                    engine_uncompressed_bytes=info["Size"], layers=image_layers, payload_rootfs=rootfs,
                    **rootfs_totals,
                    payload_rootfs_file_bytes=sum(f["size"] for f in rootfs.values() if "sha256" in f),
                    uncompressed_layer_tar_bytes=sum(l["uncompressed_tar_bytes"] for l in image_layers),
                    reencoded_gzip_layer_bytes=sum(l["reencoded_gzip_bytes"] for l in image_layers))
                if label == "base":
                    # Keep receipts, not a second multi-GB image, for comparison.
                    subprocess.run(["docker", "image", "rm", tag], check=True)
                    tags.remove(tag)
            before, after = (records["images"][n] for n in ("base", "candidate"))
            compare(before["payload_rootfs"], after["payload_rootfs"])
            for field in ("Env", "Entrypoint", "Cmd", "Healthcheck", "WorkingDir", "User", "Volumes", "ExposedPorts"):
                compare({field: before["config"].get(field)}, {field: after["config"].get(field)})
            no_payload_copyups(after["layers"])
            records["runtime"] = runtime_checks(tags[0], args.arch)
            records["result"] = "passed"
    except Exception as error:
        records["result"] = "failed"
        records["error"] = dict(type=type(error).__name__, detail=str(error)[:2000])
        raise
    finally:
        # A partial/failed measurement is retained, never labelled a pass.
        records["cleanup"] = []
        for tag in tags:
            result = subprocess.run(["docker", "image", "rm", tag], check=False)
            records["cleanup"].append(dict(image=tag, exit=result.returncode))
        cleanup_failed = any(r["exit"] for r in records["cleanup"])
        if cleanup_failed:
            records["result"] = "failed"
            records["cleanup_error"] = "owned image cleanup failed"
        args.output.write_text(json.dumps(records, indent=2)+"\n")
        if cleanup_failed:
            raise RuntimeError("owned image cleanup failed")


if __name__ == "__main__":
    main()
