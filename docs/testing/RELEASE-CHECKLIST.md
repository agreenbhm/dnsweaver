# Release Checklist

Pre-release testing protocol for dnsweaver. Every item must pass before tagging a release.

## Quick Reference

```bash
# Run the full pre-release validation locally
gofmt -w . && gofmt -l .
golangci-lint run ./...
go test ./... -count=1 -race
go build ./...
make test-integration  # Requires test environment
```

---

## 1. Automated CI Checks

These checks run automatically in the GitLab CI pipeline. Verify all pass on the release branch.

| Check | Command | Pass Criteria |
|-------|---------|---------------|
| ☐ Go format | `gofmt -l .` | No output (all files formatted) |
| ☐ Linting | `golangci-lint run ./...` | Zero issues |
| ☐ Unit tests | `go test ./... -count=1` | All pass, zero failures |
| ☐ Race detection | `go test ./... -race` | No data races detected |
| ☐ Build | `go build ./...` | Clean build, no errors |
| ☐ Docker build | `docker build .` | Image builds successfully |
| ☐ SBOM generation | CI artifact | SBOM generated for release |
| ☐ Security scan | `gitleaks detect` | No secret leaks |

## 2. Manual Integration Tests

These require a running test environment with real provider backends.

### Provider Verification

For each provider enabled in the test environment:

| Provider | Create | Update | Delete | Orphan Cleanup | Ownership |
|----------|--------|--------|--------|----------------|-----------|
| ☐ Technitium | ☐ | ☐ | ☐ | ☐ | ☐ |
| ☐ Cloudflare | ☐ | ☐ | ☐ | ☐ | ☐ |
| ☐ RFC 2136 | ☐ | ☐ | ☐ | ☐ | ☐ |
| ☐ Pi-hole v5 | ☐ | ☐ | ☐ | ☐ | ☐ |
| ☐ Pi-hole v6 | ☐ | ☐ | ☐ | ☐ | ☐ |
| ☐ dnsmasq | ☐ | ☐ | ☐ | ☐ | ☐ |
| ☐ Webhook | ☐ | ☐ | ☐ | ☐ | ☐ |

### Source Verification

For each source enabled in the test environment:

| Source | Discovery | Watch Mode | Poll Mode | Multi-Hostname |
|--------|-----------|------------|-----------|----------------|
| ☐ Traefik (Labels) | ☐ | ☐ | ☐ | ☐ |
| ☐ Traefik (File) | ☐ | ☐ | ☐ | ☐ |
| ☐ Kubernetes | ☐ | ☐ | ☐ | ☐ |
| ☐ dnsweaver Native | ☐ | ☐ | ☐ | ☐ |

### Scenario Verification

| Scenario | Status | Notes |
|----------|--------|-------|
| ☐ Service start → DNS record created | | |
| ☐ Service stop → DNS record removed | | |
| ☐ Service hostname change → old removed, new created | | |
| ☐ Service target change → record updated | | |
| ☐ dnsweaver restart → no orphans, no duplicates | | |
| ☐ Provider outage → recovery without data loss | | |

## 3. Documentation Verification

| Item | Status |
|------|--------|
| ☐ CHANGELOG.md updated with all changes since last release | |
| ☐ README.md accurate (badges, features, quick-start) | |
| ☐ Provider docs match current behavior | |
| ☐ Source docs match current behavior | |
| ☐ Configuration reference complete (all env vars documented) | |
| ☐ Docker secrets documentation current | |
| ☐ Deployment examples work with current image | |
| ☐ API version/compatibility documented (if applicable) | |

## 4. Release Artifacts

| Artifact | Status |
|----------|--------|
| ☐ Version bumped in relevant files | |
| ☐ CHANGELOG.md has release date and version header | |
| ☐ Git tag follows SemVer (`vMAJOR.MINOR.PATCH`) | |
| ☐ Docker image tagged and pushed to registry | |
| ☐ GitHub release created (if public) | |
| ☐ SBOM attached to release | |

## 5. Post-Release Verification

| Check | Status |
|-------|--------|
| ☐ Docker image pulls successfully | |
| ☐ Fresh deployment with example config works | |
| ☐ GitLab pipeline for tag completed successfully | |
| ☐ GitHub mirror release (if applicable) synced | |

---

## Version Bump Guidelines

| Change Type | Version Bump | Example |
|-------------|--------------|---------|
| Breaking API/config change | MAJOR | Removed env var, changed schema |
| New feature (backward-compatible) | MINOR | New provider, new source |
| Bug fix | PATCH | Fixed crash, corrected behavior |

See the [versioning documentation](../contributing/versioning.md) for full details.

## Release Workflow

```bash
# 1. Ensure develop is clean and all checks pass
git checkout develop
git pull origin develop
gofmt -w . && golangci-lint run ./... && go test ./... -count=1 -race && go build ./...

# 2. Merge to main
git checkout main
git pull origin main
git merge develop --no-edit

# 3. Tag the release
git tag -a v1.0.0 -m "v1.0.0 - Description of release"

# 4. Push
git push origin main --tags

# 5. Verify CI pipeline runs release jobs
# GitLab CI will: build Docker image, push to registry, create release, mirror to GitHub
```

## Hotfix Workflow

For critical fixes that can't wait for the next release:

```bash
# 1. Branch from main
git checkout -b hotfix/v1.0.1 main

# 2. Apply fix, run all checks
gofmt -w . && golangci-lint run ./... && go test ./... -count=1

# 3. Update CHANGELOG.md

# 4. Merge to main and tag
git checkout main
git merge hotfix/v1.0.1
git tag -a v1.0.1 -m "v1.0.1 - Hotfix: description"
git push origin main --tags

# 5. Backport to develop
git checkout develop
git merge hotfix/v1.0.1
git push origin develop
```
