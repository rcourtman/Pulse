#!/usr/bin/env python3
"""Cached browser OS packages reach apt only with the signed index's SHA256."""

import hashlib
import os
import stat
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
import browser_os_packages as bop  # noqa: E402


def digest(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


FAKE_APT_CACHE = """#!{python}
import sys
stanzas = [s for s in open({index!r}).read().split("\\n\\n") if s.strip()]
queries = sys.argv[2:]
open({queries!r}, "a").write("\\n".join(queries) + "\\n")
missing = False
for query in queries:
    name, _, version = query.partition("=")
    package, _, arch = name.partition(":")
    arch = arch or "amd64"
    hits = [s for s in stanzas
            if f"Package: {{package}}\\n" in s and f"Architecture: {{arch}}\\n" in s
            and (not version or f"Version: {{version}}\\n" in s)]
    if version:
        # Like apt 2.8, pkg=version selects only the first matching record.
        hits = hits[:1]
    missing = missing or not hits
    for hit in hits:
        print(hit + "\\n")
sys.exit(100 if missing else 0)
"""


class Workspace:
    """Temp cache and archive directories plus a fake apt-cache index."""

    def __init__(self, root: Path):
        self.cache = root / "cache"
        self.archives = root / "archives"
        self.cache.mkdir()
        self.archives.mkdir()
        self.index = root / "index"
        self.index.write_text("")
        self.queries = root / "queries"
        self.apt_cache = root / "apt-cache"
        # Like apt-cache show: prints every stanza a pkg[:arch] query names
        # but only the first for pkg[:arch]=version (no arch means the native
        # amd64), records the queries, and exits 100 when any is unknown.
        self.apt_cache.write_text(FAKE_APT_CACHE.format(
            python=sys.executable, index=str(self.index), queries=str(self.queries)))
        self.apt_cache.chmod(self.apt_cache.stat().st_mode | stat.S_IEXEC)

    def publish(self, package: str, version: str, arch: str, data: bytes) -> None:
        with self.index.open("a") as index:
            index.write(
                f"Package: {package}\nArchitecture: {arch}\nVersion: {version}\n"
                f"Size: {len(data)}\nSHA256: {digest(data)}\nDescription: fixture\n"
                f" continued description line\n\n"
            )

    def seed(self) -> None:
        bop.main(["seed", str(self.cache), "--archives", str(self.archives),
                  "--apt-cache", str(self.apt_cache), "--no-update"])

    def collect(self) -> str:
        output = self.cache.parent / "github-output"
        output.write_text("")
        os.environ["GITHUB_OUTPUT"] = str(output)
        try:
            bop.main(["collect", str(self.cache), "--archives", str(self.archives)])
        finally:
            del os.environ["GITHUB_OUTPUT"]
        text = output.read_text()
        return "true" if "\ntrue\n" in text else "false" if "\nfalse\n" in text else "missing"


class ArchiveNameTest(unittest.TestCase):
    def test_parses_apt_filenames_including_encoded_epochs(self):
        self.assertEqual(bop.parse_archive_name("libgtk-3-0t64_3.24.41-4ubuntu1.3_amd64.deb"),
                         ("libgtk-3-0t64", "3.24.41-4ubuntu1.3", "amd64"))
        self.assertEqual(bop.parse_archive_name("libavformat60_7%3a6.1.1-3ubuntu5_amd64.deb"),
                         ("libavformat60", "7:6.1.1-3ubuntu5", "amd64"))
        self.assertEqual(bop.parse_archive_name("fonts-noto-color-emoji_2.047-0ubuntu0.24.04.1_all.deb"),
                         ("fonts-noto-color-emoji", "2.047-0ubuntu0.24.04.1", "all"))

    def test_rejects_names_that_are_not_apt_archives(self):
        for name in ("lock", "partial", "a_b.deb", "a_b_c_d.deb", "Upper_1_amd64.deb",
                     "pkg_1 2_amd64.deb", "pkg_1_amd64.deb.part", "../x_1_amd64.deb"):
            with self.subTest(name=name):
                self.assertIsNone(bop.parse_archive_name(name))


class SeedTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.ws = Workspace(Path(self.temp.name))

    def tearDown(self):
        self.temp.cleanup()

    def test_admits_only_files_matching_the_signed_index(self):
        good = b"genuine package bytes"
        self.ws.publish("libgood", "1.0-1", "amd64", good)
        (self.ws.cache / "libgood_1.0-1_amd64.deb").write_bytes(good)
        self.ws.seed()
        admitted = self.ws.archives / "libgood_1.0-1_amd64.deb"
        self.assertEqual(admitted.read_bytes(), good)
        self.assertEqual(stat.S_IMODE(admitted.stat().st_mode), 0o644)

    def test_same_size_tampered_file_is_never_admitted(self):
        # apt itself would accept this file: it checks only the size.
        good, bad = b"genuine package bytes", b"tampered package byte"
        self.assertEqual(len(good), len(bad))
        self.ws.publish("libgood", "1.0-1", "amd64", good)
        (self.ws.cache / "libgood_1.0-1_amd64.deb").write_bytes(bad)
        self.ws.seed()
        self.assertFalse((self.ws.archives / "libgood_1.0-1_amd64.deb").exists())

    def test_versions_and_architectures_missing_from_the_index_are_skipped(self):
        self.ws.publish("libold", "2.0-1", "amd64", b"new")
        (self.ws.cache / "libold_1.0-1_amd64.deb").write_bytes(b"old")
        self.ws.publish("libforeign", "1.0-1", "arm64", b"other arch")
        (self.ws.cache / "libforeign_1.0-1_amd64.deb").write_bytes(b"other arch")
        self.ws.seed()
        self.assertEqual(list(self.ws.archives.iterdir()), [])

    def test_symlinks_and_unparsable_files_are_ignored(self):
        data = b"genuine"
        self.ws.publish("liblink", "1.0-1", "amd64", data)
        target = self.ws.cache.parent / "elsewhere.deb"
        target.write_bytes(data)
        (self.ws.cache / "liblink_1.0-1_amd64.deb").symlink_to(target)
        (self.ws.cache / "not-an-archive.deb").write_bytes(data)
        self.ws.seed()
        self.assertEqual(list(self.ws.archives.iterdir()), [])

    def test_queries_name_the_architecture_of_every_archive(self):
        # On a multiarch runner an unqualified query returns only the native
        # stanza, so a valid foreign or Architecture: all archive would be
        # downloaded again every run.
        native, foreign, common = b"amd64 build", b"i386 build", b"all build"
        self.ws.publish("libmulti", "1.0-1", "amd64", native)
        self.ws.publish("libmulti", "1.0-1", "i386", foreign)
        self.ws.publish("fonts-common", "1:2.0-1", "all", common)
        (self.ws.cache / "libmulti_1.0-1_i386.deb").write_bytes(foreign)
        (self.ws.cache / "fonts-common_1%3a2.0-1_all.deb").write_bytes(common)
        self.ws.seed()
        self.assertEqual((self.ws.archives / "libmulti_1.0-1_i386.deb").read_bytes(), foreign)
        self.assertEqual((self.ws.archives / "fonts-common_1%3a2.0-1_all.deb").read_bytes(), common)
        self.assertEqual(sorted(self.ws.queries.read_text().split()),
                         ["fonts-common:all", "libmulti:i386"])

    def test_any_repository_hash_for_the_version_is_accepted(self):
        # Two configured repositories can publish one version with different
        # bytes; a file matching either signed stanza is genuine.
        first, second = b"first repository", b"second repository"
        self.ws.publish("libdual", "1.0-1", "amd64", first)
        self.ws.publish("libdual", "1.0-1", "amd64", second)
        for variant in (first, second):
            with self.subTest(variant=variant):
                (self.ws.cache / "libdual_1.0-1_amd64.deb").write_bytes(variant)
                self.ws.seed()
                self.assertEqual((self.ws.archives / "libdual_1.0-1_amd64.deb").read_bytes(), variant)

    def test_admitted_bytes_are_the_bytes_that_were_hashed(self):
        # Swapping the cache file after it is read must not change what apt
        # gets: the verified private copy is what is installed.
        good, bad = b"genuine package bytes", b"tampered package byte"
        self.ws.publish("libgood", "1.0-1", "amd64", good)
        cached = self.ws.cache / "libgood_1.0-1_amd64.deb"
        cached.write_bytes(good)
        real_install = bop.install_file

        def swap_then_install(source, archives):
            cached.write_bytes(bad)
            real_install(source, archives)

        bop.install_file = swap_then_install
        try:
            self.ws.seed()
        finally:
            bop.install_file = real_install
        self.assertEqual((self.ws.archives / "libgood_1.0-1_amd64.deb").read_bytes(), good)

    def test_empty_cache_needs_no_index_refresh(self):
        # A missing update command would fail if seed tried to run it.
        bop.seed(self.ws.cache, self.ws.archives, str(self.ws.apt_cache), ["/nonexistent/apt-get", "update"])
        self.assertEqual(list(self.ws.archives.iterdir()), [])


class CollectTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.ws = Workspace(Path(self.temp.name))

    def tearDown(self):
        self.temp.cleanup()

    def test_first_run_saves_what_apt_downloaded(self):
        (self.ws.archives / "liba_1_amd64.deb").write_bytes(b"a")
        (self.ws.archives / "lock").write_bytes(b"")
        self.assertEqual(self.ws.collect(), "true")
        self.assertEqual(sorted(p.name for p in self.ws.cache.iterdir()), ["liba_1_amd64.deb"])

    def test_unchanged_set_is_not_saved_again(self):
        for directory in (self.ws.archives, self.ws.cache):
            (directory / "liba_1_amd64.deb").write_bytes(b"a")
        self.assertEqual(self.ws.collect(), "false")

    def test_new_and_stale_versions_refresh_the_cache(self):
        (self.ws.cache / "liba_1_amd64.deb").write_bytes(b"old")
        (self.ws.archives / "liba_2_amd64.deb").write_bytes(b"new")
        self.assertEqual(self.ws.collect(), "true")
        self.assertEqual(sorted(p.name for p in self.ws.cache.iterdir()), ["liba_2_amd64.deb"])

    def test_rejected_same_name_file_is_replaced(self):
        (self.ws.cache / "liba_1_amd64.deb").write_bytes(b"bad")
        (self.ws.archives / "liba_1_amd64.deb").write_bytes(b"gen")
        self.assertEqual(self.ws.collect(), "true")
        self.assertEqual((self.ws.cache / "liba_1_amd64.deb").read_bytes(), b"gen")

    def test_restored_symlinks_and_directories_do_not_survive_a_rebuild(self):
        outside = self.ws.cache.parent / "outside"
        outside.write_bytes(b"not apt's")
        (self.ws.cache / "liba_1_amd64.deb").symlink_to(outside)
        (self.ws.cache / "libb_1_amd64.deb").mkdir()
        (self.ws.cache / "libb_1_amd64.deb" / "inner").write_bytes(b"x")
        (self.ws.archives / "liba_1_amd64.deb").write_bytes(b"a")
        (self.ws.archives / "libb_1_amd64.deb").write_bytes(b"b")
        self.assertEqual(self.ws.collect(), "true")
        entries = sorted(self.ws.cache.iterdir())
        self.assertEqual([p.name for p in entries], ["liba_1_amd64.deb", "libb_1_amd64.deb"])
        self.assertTrue(all(p.is_file() and not p.is_symlink() for p in entries))
        self.assertEqual(outside.read_bytes(), b"not apt's")

    def test_symlinked_cache_directory_is_replaced_not_followed(self):
        elsewhere = self.ws.cache.parent / "elsewhere"
        elsewhere.mkdir()
        (elsewhere / "keep").write_bytes(b"keep")
        self.ws.cache.rmdir()
        self.ws.cache.symlink_to(elsewhere, target_is_directory=True)
        (self.ws.archives / "liba_1_amd64.deb").write_bytes(b"a")
        self.assertEqual(self.ws.collect(), "true")
        self.assertFalse(self.ws.cache.is_symlink())
        self.assertEqual([p.name for p in self.ws.cache.iterdir()], ["liba_1_amd64.deb"])
        self.assertEqual([p.name for p in elsewhere.iterdir()], ["keep"])

    def test_empty_archives_never_overwrite_the_cache(self):
        (self.ws.cache / "liba_1_amd64.deb").write_bytes(b"a")
        self.assertEqual(self.ws.collect(), "false")
        self.assertTrue((self.ws.cache / "liba_1_amd64.deb").exists())


if __name__ == "__main__":
    unittest.main()
