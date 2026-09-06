# CI benchmark evidence

`run-ci-benchmarks.sh` writes `bench-metadata.txt` beside the timing files.
The workflow uploads all four files even when the regression gate fails.
Metadata records the candidate and baseline Git HEAD and committed tree IDs,
selected Go versions, platform, GOMAXPROCS setting, sample count, duration,
package list and paired sample start order/timestamps. It deliberately does
not dump environment variables, remote URLs or filesystem paths.

HEAD/tree identify committed source, not a clean-worktree or clean-build
attestation. The workflow adds a frontend embed stub. A PR checkout may be
a synthetic merge rather than the PR head; use the captured identity, not the
run API's head SHA alone, when reproducing it. Non-Git trees and nested archives
report identity unavailable rather than inheriting an enclosing checkout's SHA.

Retain the original failed comparison. Compile both exact trees with the same
Go version before measuring, warm both, alternate their execution order and
collect at least ten samples. Record CPU/load observations and any departures
from CI (affinity, GOMAXPROCS, benchmark filtering or duration). The maintainer
heavy-work wrapper must still be used for builds; it does not prove an idle
host. A focused reproduction is diagnostic, not replacement qualification.

Unchanged source in a benchmark's file does not establish unchanged generated
code, binary layout or runtime behaviour. Matching address-normalised
instructions alone do not prove equivalent timing. Do not waive the gate from
an unrelated diff, a single passing repeat or an unverified noise hypothesis.

Each measured Go test executable is now SHA-256 hashed immediately before
execution through Go's `-exec` wrapper. Metadata `binary=label,round,name,sha256`
records bind each paired ordinal to the executed bytes; unpaired runs use
`unpaired` because Go performs all counts in one invocation. Warmups are not
recorded. Hashing is outside the benchmark timer, but it touches executable
pages and is an instrumentation change, not a controlled comparison with older
runs. Execution arguments, temporary paths and environment contents are not
recorded. A failing executable still fails collection.

These hashes do not retain binaries or establish equivalent instructions,
linked data, host state or performance. They make later reconstructions
checkable; a hash mismatch means they are not the original measured bytes.
Historical failures without hashes remain unresolved evidence.
