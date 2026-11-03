#!/usr/bin/env python3
"""Decide whether an existing proof test covers a touched runtime file.

The canonical completion guard used to accept a runtime change only when one
of the registered proof files was staged with a diff. The cheapest diff was
one more `expect(source).toContain(...)` pin, so registered proof files grew
thousands of source-text pins that proved nothing about behaviour. A proof
that already exercises the touched file and runs in CI is real verification
whether or not this commit edits it.

Coverage is a static relation between the proof file and the runtime file,
read from the tree under evaluation (the index before a commit exists, the
commit itself in CI):

- Go: the proof is a `_test.go` file compiled with the runtime file's package
  (same directory) or importing that package in its import declarations, and
  no build tag beyond a platform keeps it out of the default test run.
- TypeScript/JavaScript: the proof imports the runtime module (relative or
  `@/` specifier). Reading the module's source as text does not count, neither
  through `readFileSync` nor through a Vite `?raw` import; a type-only import
  does not run the module, and a mocked module runs the mock.
- Python: the proof imports the runtime module by name at runtime (parsed,
  so docstrings and `if TYPE_CHECKING:` imports do not count).
- Anything else (docs, workflows, shell scripts, JSON): the proof names the
  runtime file's repository path, because reading or executing that file is
  how such files are tested.

CI's required test jobs run every proof file, so "exists and covers" plus a
green CI run is the verification the guard asks for.
"""

from __future__ import annotations

import ast
import posixpath
import re
import subprocess
import sys
from pathlib import Path
from typing import Callable, Dict, Iterable, List, Optional, Sequence

from repo_file_io import git_env


REPO_ROOT = Path(__file__).resolve().parents[2]
GO_MODULE = "github.com/rcourtman/pulse-go-rewrite"
FRONTEND_SRC = "frontend-modern/src/"

JS_SUFFIXES: tuple[str, ...] = (".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs")
JS_RESOLVE_SUFFIXES: tuple[str, ...] = (
    "",
    ".ts",
    ".tsx",
    ".js",
    ".jsx",
    ".mjs",
    "/index.ts",
    "/index.tsx",
    "/index.js",
)

# `import x from 'm'`, `import {a,\n b} from "m"`, `export * from 'm'`; the
# `type` group lets type-only imports be skipped because they never run `m`.
JS_STATIC_IMPORT_RE = re.compile(
    r"""(?:^|[;\n])\s*(?:import|export)\s+(type\s+)?[^;'"]*?\bfrom\s*['"]([^'"\n]+)['"]""",
)
JS_SIDE_EFFECT_IMPORT_RE = re.compile(r"""(?:^|[;\n])\s*import\s*['"]([^'"\n]+)['"]""")
# A member call (`obj.require(...)`) or a quoted word is not a module load.
JS_DYNAMIC_IMPORT_RE = re.compile(
    r"""(?<![\w$.'"`])(?:import|require)\s*\(\s*['"]([^'"\n]+)['"]\s*\)"""
)
JS_MOCK_RE = re.compile(
    r"""\b(?:vi|jest)\.(?:mock|doMock)\s*\(\s*(?:import\s*\(\s*)?['"]([^'"\n]+)['"]"""
)
JS_BLOCK_COMMENT_RE = re.compile(r"/\*.*?\*/", re.DOTALL)
# `//` after code or at line start; `http://` and `'//cdn'` keep their text.
JS_LINE_COMMENT_RE = re.compile(r"(^|[\s;{},()])//[^\n]*", re.MULTILINE)
JS_TYPE_QUERY_RE = re.compile(r"\btypeof\s+import\s*\(")
GO_BLOCK_COMMENT_RE = re.compile(r"/\*.*?\*/", re.DOTALL)
GO_IMPORT_BLOCK_RE = re.compile(r"^import\s*\((.*?)\)", re.MULTILINE | re.DOTALL)
GO_IMPORT_LINE_RE = re.compile(r'^import\s+(?:[\w.]+\s+)?"([^"]+)"', re.MULTILINE)
GO_IMPORT_SPEC_RE = re.compile(r'^\s*(?:[\w.]+\s+)?"([^"]+)"', re.MULTILINE)
GO_BUILD_RE = re.compile(r"^//go:build\s+(.+)$", re.MULTILINE)
# Constraints CI's Go jobs satisfy or that only pick a platform; any other tag
# (`ignore`, `integration`, ...) can keep a test out of the required run.
GO_PLATFORM_TAGS = frozenset(
    {
        "386", "amd64", "arm", "arm64", "cgo", "darwin", "freebsd", "linux",
        "netbsd", "openbsd", "unix", "windows",
    }
)
PATH_CHAR = r"[\w./-]"


TextReader = Callable[[str], Optional[str]]


class GitTextReader:
    """Read files from a commit, or from the index when `revision` is None.

    One `git cat-file --batch` process serves every read, so checking a
    requirement's proofs costs one round trip per file rather than a process.
    """

    def __init__(self, revision: str | None, *, repo_root: Path = REPO_ROOT) -> None:
        self.prefix = f"{revision}:" if revision is not None else ":0:"
        self.repo_root = repo_root
        self.process: subprocess.Popen | None = None
        self.cache: Dict[str, Optional[str]] = {}

    def __call__(self, path: str) -> Optional[str]:
        if path in self.cache:
            return self.cache[path]
        if self.process is None:
            self.process = subprocess.Popen(
                ["git", "cat-file", "--batch"],
                cwd=self.repo_root,
                stdin=subprocess.PIPE,
                stdout=subprocess.PIPE,
                env=git_env(self.repo_root),
            )
        assert self.process.stdin is not None and self.process.stdout is not None
        self.process.stdin.write(f"{self.prefix}{path}\n".encode())
        self.process.stdin.flush()
        header = self.process.stdout.readline().decode("utf-8", errors="replace").split()
        text: Optional[str] = None
        if len(header) == 3:
            # Every found object is followed by its body; a tree (a directory
            # a specifier names) must be drained too or the stream desyncs.
            body = self.process.stdout.read(int(header[2]))
            self.process.stdout.read(1)
            if header[1] == "blob":
                text = body.decode("utf-8", errors="replace")
        self.cache[path] = text
        return text

    def close(self) -> None:
        if self.process is None:
            return
        assert self.process.stdin is not None and self.process.stdout is not None
        self.process.stdin.close()
        self.process.wait()
        self.process.stdout.close()
        self.process = None

    def __enter__(self) -> "GitTextReader":
        return self

    def __exit__(self, *_exc: object) -> None:
        self.close()


def working_tree_reader(repo_root: Path = REPO_ROOT) -> TextReader:
    """Read files as they are on disk, for lookups made while editing."""

    def read(path: str) -> Optional[str]:
        try:
            return (repo_root / path).read_text(encoding="utf-8", errors="replace")
        except (FileNotFoundError, IsADirectoryError, NotADirectoryError):
            return None

    return read


def git_tracked_paths(revision: str | None, prefixes: Sequence[str], *, repo_root: Path = REPO_ROOT) -> List[str]:
    """Paths under any of `prefixes` in a commit, or in the index."""
    if not prefixes:
        return []
    if revision is None:
        command = ["git", "ls-files", "-z", "--", *prefixes]
    else:
        command = ["git", "ls-tree", "-r", "-z", "--name-only", revision, "--", *prefixes]
    result = subprocess.run(
        command,
        cwd=repo_root,
        check=True,
        capture_output=True,
        env=git_env(repo_root),
    )
    return sorted(path for path in result.stdout.decode().split("\0") if path)


def is_js_path(path: str) -> bool:
    return path.endswith(JS_SUFFIXES) and not path.endswith(".d.ts")


def js_code(text: str) -> str:
    """Drop comments and `typeof import(...)` type queries, which never run a module.

    Stripping is conservative: a `/*` inside a string can only hide imports,
    never invent one.
    """
    text = JS_BLOCK_COMMENT_RE.sub("", text)
    text = JS_LINE_COMMENT_RE.sub(r"\1", text)
    return JS_TYPE_QUERY_RE.sub("typeof ", text)


def js_import_specifiers(text: str) -> List[str]:
    specifiers = [spec for type_only, spec in JS_STATIC_IMPORT_RE.findall(text) if not type_only]
    specifiers.extend(JS_SIDE_EFFECT_IMPORT_RE.findall(text))
    specifiers.extend(JS_DYNAMIC_IMPORT_RE.findall(text))
    return specifiers


def js_specifier_targets(importer: str, specifier: str) -> List[str]:
    """Candidate repository paths a module specifier can resolve to, in order.

    A query suffix (`?raw`, `?url`, `?inline`) loads the file as text or an
    asset without running it, which is a source pin, not coverage.
    """
    if "?" in specifier:
        return []
    if specifier.startswith("@/"):
        base = FRONTEND_SRC + specifier[2:]
    elif specifier.startswith("."):
        base = posixpath.normpath(posixpath.join(posixpath.dirname(importer), specifier))
    else:
        return []
    return [base + suffix for suffix in JS_RESOLVE_SUFFIXES]


def resolve_js_specifier(importer: str, specifier: str, read: TextReader) -> Optional[str]:
    """The one file a specifier loads: the first candidate that exists."""
    return next(
        (path for path in js_specifier_targets(importer, specifier) if read(path) is not None),
        None,
    )


def go_package_dir(path: str) -> str:
    return posixpath.dirname(path)


def go_imported_packages(text: str) -> set[str]:
    """Import paths from the file's import declarations, not any quoted string."""
    code = GO_BLOCK_COMMENT_RE.sub("", text)
    packages = set(GO_IMPORT_LINE_RE.findall(code))
    for block in GO_IMPORT_BLOCK_RE.findall(code):
        packages.update(GO_IMPORT_SPEC_RE.findall(block))
    return packages


def go_test_runs_by_default(text: str) -> bool:
    """False when a build constraint needs a tag CI's Go test jobs do not set."""
    header = text.split("\npackage ", 1)[0]
    for expression in GO_BUILD_RE.findall(header):
        for tag in re.findall(r"[\w.]+", expression):
            if tag not in GO_PLATFORM_TAGS and not re.fullmatch(r"go1\.\d+", tag):
                return False
    return True


def python_module_names(path: str) -> List[str]:
    """Names a Python test could import the runtime module by.

    A bare name that is also a standard-library module (`json`, `types`)
    would name the stdlib module, not this file.
    """
    stem = posixpath.splitext(path)[0]
    parts = stem.split("/")
    if parts[-1] == "__init__":
        parts = parts[:-1]
    stdlib = set(getattr(sys, "stdlib_module_names", ()))
    names = [".".join(parts[index:]) for index in range(len(parts))]
    return [name for name in names if "." in name or name not in stdlib]


def _is_type_checking(test: ast.expr) -> bool:
    return (isinstance(test, ast.Name) and test.id == "TYPE_CHECKING") or (
        isinstance(test, ast.Attribute) and test.attr == "TYPE_CHECKING"
    )


def python_imported_modules(text: str) -> set[str]:
    """Modules a Python file imports at runtime, including `from pkg import module`.

    Docstrings, comments and `if TYPE_CHECKING:` imports do not count.
    """
    try:
        tree = ast.parse(text)
    except SyntaxError:
        return set()
    type_only: set[int] = set()
    for node in ast.walk(tree):
        if isinstance(node, ast.If) and _is_type_checking(node.test):
            for child in node.body:
                type_only.update(id(inner) for inner in ast.walk(child))
    modules: set[str] = set()
    for node in ast.walk(tree):
        if id(node) in type_only:
            continue
        if isinstance(node, ast.Import):
            modules.update(alias.name for alias in node.names)
        elif isinstance(node, ast.ImportFrom) and node.module and not node.level:
            modules.add(node.module)
            modules.update(f"{node.module}.{alias.name}" for alias in node.names)
        elif (
            isinstance(node, ast.Call)
            and isinstance(node.func, (ast.Attribute, ast.Name))
            and getattr(node.func, "attr", getattr(node.func, "id", "")) == "import_module"
            and node.args
            and isinstance(node.args[0], ast.Constant)
            and isinstance(node.args[0].value, str)
        ):
            modules.add(node.args[0].value)
    return modules


def names_repo_path(text: str, path: str) -> bool:
    """The text names `path` itself, not a longer path that ends with it."""
    return re.search(rf"(?<!{PATH_CHAR}){re.escape(path)}(?!{PATH_CHAR})", text) is not None


def proof_covers_runtime_file(proof: str, runtime: str, read: TextReader) -> bool:
    """Return True when the proof file, as it exists, exercises `runtime`."""
    if proof == runtime:
        return False
    text = read(proof)
    if text is None or read(runtime) is None:
        return False

    if runtime.endswith(".go"):
        if not proof.endswith("_test.go") or not go_test_runs_by_default(text):
            return False
        if go_package_dir(proof) == go_package_dir(runtime):
            return True
        return f"{GO_MODULE}/{go_package_dir(runtime)}" in go_imported_packages(text)

    if is_js_path(runtime):
        if not is_js_path(proof):
            return False
        code = js_code(text)
        # A mocked module is replaced, so importing it runs the mock instead.
        if any(resolve_js_specifier(proof, spec, read) == runtime for spec in JS_MOCK_RE.findall(code)):
            return False
        return any(
            resolve_js_specifier(proof, specifier, read) == runtime
            for specifier in js_import_specifiers(JS_MOCK_RE.sub("", code))
        )

    if runtime.endswith(".py"):
        if not proof.endswith(".py"):
            return False
        imported = python_imported_modules(text)
        return any(name in imported for name in python_module_names(runtime))

    return names_repo_path(text, runtime)


def covering_proof_files(
    runtime: str,
    candidate_proofs: Iterable[str],
    read: TextReader,
) -> List[str]:
    return sorted(
        {proof for proof in candidate_proofs if proof_covers_runtime_file(proof, runtime, read)}
    )
