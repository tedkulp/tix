---
name: tix-release
description: Use when releasing a new version of tix — updating the changelog, tagging, and pushing to trigger GoReleaser.
---

# tix Release

## Overview

Releases are triggered by pushing a version tag. GoReleaser builds binaries, publishes a GitHub Release, and updates the Homebrew tap automatically. Your job is to prepare the repo and tag correctly.

## Release Process

### Step 1: Determine the version

Check the latest tag and decide the next version using semver:

```bash
git tag --sort=-version:refname | head -5
```

- **Patch** (`0.6.5 → 0.6.6`): bug fixes only
- **Minor** (`0.6.x → 0.7.0`): new features, backwards-compatible
- **Major** (`0.x.x → 1.0.0`): breaking changes

### Step 2: Update CHANGELOG.md

The file uses [Keep a Changelog](https://keepachangelog.com) format.

1. Find the `## [Unreleased]` section
2. Add a new versioned section **above** it using today's date:

```markdown
## [Unreleased]

## [0.6.6] - 2026-03-10

### Added
- ...

### Fixed
- ...
```

3. Leave `## [Unreleased]` empty (do not remove it)
4. Sections to use: `Added`, `Changed`, `Deprecated`, `Removed`, `Fixed`, `Security`

### Step 3: Commit

```bash
git add CHANGELOG.md
git commit -m "Release v0.6.6: <one-line summary of what changed>"
```

Commit message format: `Release vX.Y.Z: <short description>` — match existing history.

### Step 4: Tag

```bash
git tag v0.6.6
```

### Step 5: Push commit and tag

```bash
git push && git push origin v0.6.6
```

Pushing the tag triggers the `release.yml` GitHub Actions workflow which runs GoReleaser.

### Step 6: Verify

```bash
gh run list --limit 5
```

Monitor the release workflow. GoReleaser will:
- Build binaries for linux/darwin/windows (amd64 + arm64)
- Publish a GitHub Release with assets and checksums
- Update the Homebrew tap (`tedkulp/homebrew-tap`)

## Quick Reference

| Step | Command |
|------|---------|
| Check latest tags | `git tag --sort=-version:refname \| head -5` |
| Commit | `git commit -m "Release vX.Y.Z: <summary>"` |
| Tag | `git tag vX.Y.Z` |
| Push both | `git push && git push origin vX.Y.Z` |
| Check CI | `gh run list --limit 5` |

## Common Mistakes

- **Forgetting to push the tag separately** — `git push` alone does not push tags; you need `git push origin vX.Y.Z`
- **Removing `[Unreleased]`** — always leave it in place, just empty
- **Wrong date** — use today's actual date in `YYYY-MM-DD` format
- **Tagging before committing CHANGELOG** — commit first, then tag so the tag includes the changelog update
