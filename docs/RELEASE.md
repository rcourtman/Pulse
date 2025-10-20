# Pulse Release Checklist

Use this checklist when preparing and publishing a new Pulse release.

## Pre-release

- [ ] Ensure `VERSION` is set to `4.24.0` and matches the tag you plan to cut (format `4.x.y`)
- [ ] Confirm the Helm chart renders and installs locally:
  ```bash
  helm lint deploy/helm/pulse --strict
  helm template pulse deploy/helm/pulse \
    --set persistence.enabled=false \
    --set server.secretEnv.create=true \
    --set server.secretEnv.data.API_TOKENS=dummy-token
  ```
- [ ] (Optional) Run the Kind-based integration test locally:
  ```bash
  kind create cluster
  helm upgrade --install pulse ./deploy/helm/pulse \
    --namespace pulse \
    --create-namespace \
    --set persistence.enabled=false \
    --set server.secretEnv.create=true \
    --set server.secretEnv.data.API_TOKENS=dummy-token \
    --wait
  kubectl -n pulse get pods
  kind delete cluster
  ```
- [ ] Confirm adaptive polling, scheduler health API, rollback UI, logging runtime controls, and rate-limit header documentation are updated before tagging v4.24.0
- [ ] Smoke-test updates rollback: apply a test update via Settings → System → Updates, trigger a rollback, and verify journal entries document the rollback event

## Publishing

1. Tag the release (`git tag v4.x.y && git push origin v4.x.y`) or draft a GitHub release.

2. Package the Helm chart locally so you can preview the artifact (the GitHub workflow performs the same command, but local packaging provides an explicit hand-off):
   ```bash
   ./scripts/package-helm-chart.sh 4.x.y
   # Optional: push to GHCR after authenticating
   # helm registry login ghcr.io
   # ./scripts/package-helm-chart.sh 4.x.y --push
   ```
   The script emits `dist/pulse-4.x.y.tgz`, and `scripts/build-release.sh` copies the tarball into `release/` alongside the binary archives. Uploading can be handled manually with the `--push` flag or delegated to the automated workflow described below.
   > `scripts/build-release.sh` automatically runs the same packaging step (unless you export `SKIP_HELM_PACKAGE=1`) so release archives and chart tarballs are produced together.

3. If you rely on automation, monitor the **Publish Helm Chart** workflow (triggered by the release) to ensure it finishes successfully. When running entirely locally, skip this step and verify the push command completed.

4. (Optional) Sign `release/checksums.txt` by exporting `SIGNING_KEY_ID=<gpg-key-id>` before running `scripts/build-release.sh`, or re-run the signing step manually:
   ```bash
   SIGNING_KEY_ID=<gpg-key-id> ./scripts/build-release.sh
   # or sign later
   gpg --detach-sign --armor --local-user <gpg-key-id> release/checksums.txt
   ```
   Publish both `checksums.txt` and `checksums.txt.asc` so users can verify artifacts:
   ```bash
   gpg --verify checksums.txt.asc checksums.txt
   ```

5. Update the release notes to include an upgrade/install snippet pointing at GHCR, for example:
   ```bash
   helm install pulse oci://ghcr.io/rcourtman/pulse-chart \
     --version 4.x.y \
     --namespace pulse \
     --create-namespace
   ```

   **For v4.24.0 specifically**, highlight these features in the release notes:
   - Adaptive polling (now GA)
   - Scheduler health API with rich instance metadata
   - Updates rollback workflow
   - Shared script library system (now GA)
   - X-RateLimit-* headers for all API responses
   - Runtime logging configuration (no restart required)

6. Mention any chart-breaking changes (new values, migrations) in the release notes.

## Post-release

- [ ] Verify `helm show chart oci://ghcr.io/rcourtman/pulse-chart --version 4.x.y` shows the expected metadata (version, appVersion, icon)
- [ ] Run `helm install` against a test cluster (Kind/k3s) using the published OCI artifact
- [ ] Run `curl -s http://<host>:7655/api/monitoring/scheduler/health | jq` to ensure the scheduler health endpoint is live
- [ ] Verify the Updates view reports rollback metadata and X-RateLimit-* headers appear in API responses
- [ ] Announce the release with links to both the GitHub release and the Helm installation instructions (`docs/KUBERNETES.md`)
- [ ] Verify signatures: `gpg --verify checksums.txt.asc checksums.txt`
