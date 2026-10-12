import unittest
from pathlib import Path
import socket
import tempfile

from artifact_redaction import MAX_ARTIFACT_TEXT, assert_allowed_artifact_tree, contains_forbidden_secret_shape, redact_text, verify_checksums, write_checksums


class ArtifactRedactionTest(unittest.TestCase):
    def test_redacts_bearer_and_named_secret(self) -> None:
        redacted = redact_text("Authorization: Bearer sensitive\napi_token=private")
        self.assertNotIn("sensitive", redacted)
        self.assertNotIn("private", redacted)

    def test_bounds_output(self) -> None:
        self.assertEqual(len(redact_text("x" * (MAX_ARTIFACT_TEXT + 20))), MAX_ARTIFACT_TEXT)

    def test_rejects_unredacted_secret_shape(self) -> None:
        self.assertTrue(contains_forbidden_secret_shape("Authorization: Bearer value"))

    def test_strict_allowlist_rejects_runtime_state_and_unknown_files(self) -> None:
        for name in ("state.db", "state.db-wal", "state.sqlite", "state.sqlite3", "state.wal", "state.shm", "runtime.log", ".env", "token.txt", "cookie.json", "unknown.txt"):
            with self.subTest(name=name), tempfile.TemporaryDirectory() as directory:
                (Path(directory) / name).write_text("x", encoding="utf-8")
                with self.assertRaises(ValueError):
                    assert_allowed_artifact_tree(Path(directory))

    def test_strict_allowlist_rejects_socket_and_subdirectory(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            sock = socket.socket(socket.AF_UNIX)
            try:
                sock.bind(str(root / "runtime.sock"))
                with self.assertRaises(ValueError):
                    assert_allowed_artifact_tree(root)
            finally:
                sock.close()
        with tempfile.TemporaryDirectory() as directory:
            (Path(directory) / "nested").mkdir()
            with self.assertRaises(ValueError):
                assert_allowed_artifact_tree(Path(directory))

    def test_allowed_manifest_round_trip(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "environment.json").write_text("{}\n", encoding="utf-8")
            (root / "run-report.json").write_text("{}\n", encoding="utf-8")
            write_checksums(root)
            verify_checksums(root)


if __name__ == "__main__":
    unittest.main()
