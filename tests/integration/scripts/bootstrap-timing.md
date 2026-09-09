# Settings bootstrap timing diagnostic

The desktop-to-390px Settings test attaches `bootstrap-paired-timing` to its
Playwright report, including on assertion failure. It issues at most one extra
GET each for summary and runtime capabilities, when the browser first requests
that path. The HTTP client shares the test's cookie session but not the browser's
network scheduler. Redirect following is disabled; each probe has a five-second
limit. No readiness condition, capability requirement or assertion is relaxed.

`started` is host epoch milliseconds. Browser duration ends at response headers;
HTTP duration includes response receipt. Null browser status/duration is censored,
not a server error. Null HTTP status is a transport failure/timeout without a
received response; the exception is deliberately not retained. Reports exclude
headers, credentials, query strings and payloads.

For the next hosted observation, retain this attachment, the browser trace and
application startup timestamps from the same job/source. Compare overlapping
summary/capabilities requests; a quick HTTP response with stalled browser narrows
the question towards browser scheduling, while both delayed supports further
server/startup measurement. Neither proves a particular lock is responsible.
Extra probes can warm caches or contend themselves, so do not claim a controlled
effect size or historical causality. Do not select favourable repeats, widen the
assertion timeout, or treat a successful diagnostic as probation promotion.
