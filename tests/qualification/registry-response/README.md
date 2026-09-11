# Registry fallback response qualification

This synthetic proof covers the Docker agent's registry response fallback,
container collection, report ingestion and `GET /api/resources`. It does not
contact a registry, run Docker, update a container or establish an affected
installation's cause. Registry fixtures use the reported references
`redis:8-alpine`, `postgres:16-alpine`, `ghcr.io/searxng/searxng:latest` and
`ghcr.io/paperless-ngx/paperless-ngx:latest` with synthetic digest values.

The HTTP fixture first returns the same headerless HTTP 200 error page for
three references while the fourth is healthy. Those responses must produce
errors, no latest digest and no update indication, including cached checks.
A fresh checker then models agent restart with valid independent indexes;
matching index **or** platform digests must suppress updates. Exact request
paths and one HEAD/GET pair per reference establish reference isolation and
cache reuse. The production collector, not a hand-written status fixture,
produces the reports consumed by the next two probes.

Run from the repository root, using a fresh private temporary directory:

```sh
proof=$(mktemp -d)
export PULSE_REGISTRY_REPORT_PROOF="$proof/reports.json"
export PULSE_REGISTRY_SNAPSHOT_PROOF="$proof/snapshots.json"
go test -race ./internal/dockeragent -run 'TestCollectRegistryResponseIsolation|TestRegistryChecker' -count=1
go test -race ./internal/monitoring -run '^TestDockerRegistryReportQualification$' -count=1
go test -race ./internal/api/resourceapi -run '^TestResourcesRegistryResponseQualification$' -count=1 -v
```

The latter two probes skip without explicit input paths; they fail for missing
or malformed input when configured. Preserve both JSON files and command logs.
The endpoint probe calls the resource query handler directly, not the HTTP
authentication middleware. No credential or authorization acceptance is claimed.

`fetchManifest` requires a schema-2 manifest envelope (config digest or index
manifest array) before using a GET fallback body or its digest metadata. This
is a minimal error-page discriminator, not full descriptor/cryptographic
validation. See the [OCI manifest specification](https://github.com/opencontainers/image-spec/blob/main/manifest.md)
and [image index specification](https://github.com/opencontainers/image-spec/blob/main/image-index.md).
Existing HEAD digest handling, credentials, TLS, cache intervals and HTTP
error handling remain unchanged. A failed check is not proof of an up-to-date
image; consumers must retain the error field. No new UI state is introduced.

Before attributing an incident to this mechanism, obtain safe response evidence
from the affected agent's network path: status, content type, digest headers,
and locally computed body hash, excluding authorization headers, cookies and
private response content. A matching synthetic symptom alone does not prove
that the reporter received the same response. Installed acceptance requires an
updated agent and verified error/recovery status on the same workload.
