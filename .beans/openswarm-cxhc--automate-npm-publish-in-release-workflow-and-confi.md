---
# openswarm-cxhc
title: Automate npm publish in release workflow and confirm package ownership
status: completed
type: task
priority: high
created_at: 2026-04-02T13:27:34Z
updated_at: 2026-04-02T13:33:23Z
---

The npm package 'openswarm' exists on the registry (currently at 0.1.7, maintained by alexngai/alex@sudocode.ai) but the release.yml workflow has no npm publish step — it only runs GoReleaser for binaries and Homebrew. npm/package.json is pinned at 0.1.1 and is never updated automatically.

## Issues found

- `.github/workflows/release.yml` uses GoReleaser but has no `npm publish` step
- `npm/package.json` version (0.1.1) is out of sync with the registry (0.1.7)
- No automation updates `npm/package.json` version to match the git tag before publishing
- Need to confirm `NPM_TOKEN` for the `justestif` account is set in GitHub Actions secrets

## Tasks

- [ ] Confirm `NPM_TOKEN` for the `justestif` npm account is set in GitHub Actions repo secrets
- [ ] Add a step to `release.yml` that updates `npm/package.json` version to match the git tag
- [ ] Add `npm publish` step to `release.yml` after GoReleaser (so the binary artifacts exist before postinstall.js tries to download them)
- [ ] Test with a dry-run: `npm publish --dry-run` from `npm/`
- [ ] Verify `npm install -g openswarm` downloads the correct binary after a release
ify `npm install -g openswarm` downloads the correct binary after a release

## Changes made

- Renamed package from `openswarm` → `@justestif/openswarm` in npm/package.json, npm/README.md, npm/bin/swarm.js, README.md
- Added `npm-publish` job to `.github/workflows/release.yml` (runs after `goreleaser`, sets version from tag, publishes with `--access public`)

## Remaining

- [ ] Confirm `NPM_TOKEN` for the `justestif` npm account is set in GitHub Actions repo secrets
- [ ] Test with a dry-run: `npm publish --dry-run` from `npm/` after the next release
- [ ] Verify `npm install -g @justestif/openswarm` downloads the correct binary after a release

## Status update (2026-04-02)

- [x] `@justestif/openswarm@0.1.1` published manually via npm OIDC browser login — package is live
- [x] Workflow updated to use npm OIDC trusted publishing (`--provenance`, `id-token: write`, no NPM_TOKEN secret)

## One remaining prerequisite before automated publish works

Configure GitHub Actions as a trusted publisher on npmjs.com:
1. Go to https://www.npmjs.com/package/@justestif/openswarm → Publishing → Add a publisher → GitHub Actions
2. Set: owner = `justEstif`, repo = `openswarm`, workflow = `release.yml`

Once that is done, pushing a `v*` tag will fully automate the npm publish with no stored secrets.
