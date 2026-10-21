package db

import (
	"database/sql"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

var cacheSizePattern = regexp.MustCompile(`(?i)cache_size\s*(?:\(\s*|=\s*)(-?\d+)`)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate test file")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

// Every production SQLite store must take its page cache from
// PageCachePragma. A private larger cache_size costs about twice its size in
// resident memory outside the Go heap (#1803).
func TestNoStoreSetsItsOwnPageCache(t *testing.T) {
	root := repoRoot(t)
	own := filepath.Join(root, "pkg", "db", "pagecache.go")
	var offenders []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "vendor", "worktrees", "testdata":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") || path == own {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, m := range cacheSizePattern.FindAllStringSubmatch(string(body), -1) {
			rel, _ := filepath.Rel(root, path)
			offenders = append(offenders, fmt.Sprintf("%s: cache_size %s", rel, m[1]))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(offenders) > 0 {
		t.Fatalf("SQLite stores must use pdb.PageCachePragma, not their own cache_size:\n%s", strings.Join(offenders, "\n"))
	}
}

func TestPageCachePragmaStaysWithinBudget(t *testing.T) {
	m := cacheSizePattern.FindStringSubmatch(PageCachePragma)
	if m == nil {
		t.Fatalf("PageCachePragma %q is not a cache_size pragma", PageCachePragma)
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		t.Fatal(err)
	}
	if n >= 0 || -n > MaxPageCacheKiB {
		t.Fatalf("PageCachePragma requests %d; want a KiB size of at most %d (negative value)", n, MaxPageCacheKiB)
	}
}

func residentKiB(t *testing.T) int {
	t.Helper()
	body, err := os.ReadFile("/proc/self/status")
	if err != nil {
		t.Skip("resident memory is only measurable through /proc")
	}
	for _, line := range strings.Split(string(body), "\n") {
		if rest, ok := strings.CutPrefix(line, "VmRSS:"); ok {
			kib, err := strconv.Atoi(strings.TrimSuffix(strings.TrimSpace(rest), " kB"))
			if err != nil {
				t.Fatal(err)
			}
			return kib
		}
	}
	t.Skip("VmRSS unavailable")
	return 0
}

// Reading a database far larger than the cache must not grow resident memory
// by more than the bounded cache's real cost. A 64 MB cache_size grew it by
// about 117 MB in the same scan.
func TestPageCacheBoundsResidentMemory(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a 30 MB database")
	}
	path := filepath.Join(t.TempDir(), "pagecache.db")
	writer, err := sql.Open("sqlite", path+"?"+url.Values{"_pragma": {"journal_mode(WAL)", PageCachePragma}}.Encode())
	if err != nil {
		t.Fatal(err)
	}
	writer.SetMaxOpenConns(1)
	if _, err := writer.Exec(`CREATE TABLE t (id INTEGER PRIMARY KEY, a TEXT, b TEXT); CREATE INDEX t_a ON t(a)`); err != nil {
		t.Fatal(err)
	}
	tx, err := writer.Begin()
	if err != nil {
		t.Fatal(err)
	}
	pad := strings.Repeat("x", 200)
	for i := 0; i < 120000; i++ {
		if _, err := tx.Exec(`INSERT INTO t (a, b) VALUES (?, ?)`, fmt.Sprintf("k%09d", (i*7919)%120000), pad); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	reader, err := sql.Open("sqlite", path+"?"+url.Values{"_pragma": {PageCachePragma}}.Encode())
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	reader.SetMaxOpenConns(1)
	var effective int
	if err := reader.QueryRow(`PRAGMA cache_size`).Scan(&effective); err != nil {
		t.Fatal(err)
	}
	if effective != -MaxPageCacheKiB {
		t.Fatalf("effective cache_size = %d, want %d", effective, -MaxPageCacheKiB)
	}

	before := residentKiB(t)
	var rows int
	var total int64
	for pass := 0; pass < 2; pass++ {
		if err := reader.QueryRow(`SELECT count(*), sum(length(b)) FROM t`).Scan(&rows, &total); err != nil {
			t.Fatal(err)
		}
		if err := reader.QueryRow(`SELECT count(*) FROM t WHERE a > 'k'`).Scan(&rows); err != nil {
			t.Fatal(err)
		}
	}
	grew := residentKiB(t) - before
	// About twice the cache size, plus allocator and statement slack.
	if limit := 4*MaxPageCacheKiB + 8192; grew > limit {
		t.Fatalf("scanning a 30 MB database grew resident memory by %d KiB; the page cache budget allows %d KiB", grew, limit)
	}
}
