# Pre-Release Checklist for Pulse Go Rewrite

## Build Tests
- [x] Go backend builds successfully: `go build ./cmd/pulse`
- [x] Frontend builds successfully: `cd frontend-modern && npm run build`
- [x] Application starts and loads configuration
- [ ] Docker image builds: `docker build -t pulse:test .`
- [ ] Docker container runs: `docker run --rm pulse:test ./pulse --help`

## Functionality Tests
- [ ] Frontend loads at http://localhost:7655
- [ ] Backend API responds at http://localhost:3000
- [ ] Can add/edit/delete Proxmox nodes through UI
- [ ] Real-time monitoring updates work
- [ ] Alerts trigger correctly
- [ ] Email notifications work
- [ ] Webhook notifications work
- [ ] Configuration is properly encrypted

## Deployment Tests
- [ ] Docker Compose works: `docker-compose up -d`
- [ ] Systemd services work correctly
- [ ] Application restarts preserve configuration
- [ ] Updates don't lose data

## Documentation
- [x] README.md is complete and accurate
- [x] SECURITY.md explains encryption
- [x] No sensitive information in repo
- [x] All internal dev docs removed

## Final Checks
- [ ] All tests pass
- [ ] No debug code left in
- [ ] Version number updated
- [ ] Git history is clean (no sensitive data)
- [ ] Repository size is reasonable (<5MB excluding .git)

## Docker Testing Commands

```bash
# Build test
docker build -t pulse:test .

# Run test
docker run --rm -it pulse:test ./pulse --help

# Full test with volumes
docker run -d \
  --name pulse-test \
  -p 7655:7655 \
  -v pulse_test_config:/etc/pulse \
  -v pulse_test_data:/data \
  pulse:test

# Check logs
docker logs pulse-test

# Clean up
docker stop pulse-test
docker rm pulse-test
docker volume rm pulse_test_config pulse_test_data
```