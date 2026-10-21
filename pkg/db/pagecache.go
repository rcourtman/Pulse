package db

// PageCachePragma bounds each SQLite connection's private page cache. Pass it
// in a store's DSN `_pragma` list instead of writing its own cache_size.
//
// The modernc driver allocates this cache with anonymous mmap, outside the Go
// heap, so neither the garbage collector nor GOMEMLIMIT can see or reclaim it,
// and each cached page costs about twice its size once the allocator rounds it
// up. A 64 MB cache_size held about 120 MB of resident memory on a large
// database (#1803). Pages that miss this cache are re-read from the kernel's
// file cache, which the kernel can reclaim under memory pressure. This matches
// SQLite's own default of 2000 KiB.
//
// modernc applies a DSN's _pragma values in sorted order, not the order given,
// so a second cache_size in the same DSN cannot reliably override this one.
const PageCachePragma = "cache_size(-2000)"

// MaxPageCacheKiB is the largest cache_size, in KiB, that a store may request.
// The pagecache guard test fails on any larger literal in production code.
const MaxPageCacheKiB = 2000
