import json
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from release_candidate_manifest import (
    create_manifest,
    load_release_assets,
    verify_local,
    verify_release,
)


SOURCE_SHA = "1" * 40


class ReleaseCandidateManifestTest(unittest.TestCase):
    def create_release_dir(self, root: Path) -> Path:
        release_dir = root / "release"
        release_dir.mkdir()
        (release_dir / "checksums.txt").write_text("abc\n", encoding="utf-8")
        (release_dir / "pulse-v6.1.0-linux-amd64.tar.gz").write_bytes(b"archive")
        return release_dir

    def test_create_and_verify_local_candidate(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            release_dir = self.create_release_dir(Path(temp_dir))
            manifest = create_manifest(release_dir, "6.1.0", SOURCE_SHA)

            self.assertEqual(manifest["tag"], "v6.1.0")
            self.assertEqual(
                [asset["name"] for asset in manifest["assets"]],
                ["checksums.txt", "pulse-v6.1.0-linux-amd64.tar.gz"],
            )
            verify_local(release_dir, manifest, "6.1.0", SOURCE_SHA)

    def test_verify_local_rejects_tampered_asset(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            release_dir = self.create_release_dir(Path(temp_dir))
            manifest = create_manifest(release_dir, "6.1.0-rc.1", SOURCE_SHA)
            (release_dir / "checksums.txt").write_text("xyz\n", encoding="utf-8")

            with self.assertRaisesRegex(ValueError, "digest mismatch"):
                verify_local(release_dir, manifest, "6.1.0-rc.1", SOURCE_SHA)

    def test_verify_release_uses_server_side_digests(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            release_dir = self.create_release_dir(Path(temp_dir))
            manifest = create_manifest(release_dir, "6.1.0", SOURCE_SHA)
            release_assets = [
                {
                    "name": asset["name"],
                    "size": asset["size"],
                    "digest": f"sha256:{asset['sha256']}",
                }
                for asset in manifest["assets"]
            ]

            verify_release(manifest, release_assets)
            release_assets[0]["digest"] = "sha256:" + "0" * 64
            with self.assertRaisesRegex(ValueError, "published asset digest mismatch"):
                verify_release(manifest, release_assets)

    def test_release_metadata_loader_flattens_paginated_arrays(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            path = Path(temp_dir) / "assets.json"
            path.write_text(
                json.dumps([[{"name": "a"}], [{"name": "b"}]]),
                encoding="utf-8",
            )
            self.assertEqual(
                [asset["name"] for asset in load_release_assets(path)],
                ["a", "b"],
            )


if __name__ == "__main__":
    unittest.main()
