# Release Process

This document describes the release process for Tasklog using [changie](https://changie.dev/) for changelog management and GitHub Actions for automation.

## Overview

Tasklog uses a semi-automated release process:
- **Changie** manages changelog entries and versioning
- **GitHub Actions** automates release preparation and publishing
- **GoReleaser** builds and publishes binaries, Docker images, and GitHub releases

## Quick Release Guide

### For Maintainers

1. **Add changelog entries** during development (see [Adding Changelog Entries](#adding-changelog-entries))
2. **Prepare release** using GitHub Actions workflow
3. **Review and merge** the auto-generated pull request
4. **Publish release** automatically via GitHub Actions

## Adding Changelog Entries

When making changes, add a changelog entry using changie:

```bash
# Create a new changelog entry
changie new

# Follow the prompts:
# 1. Select kind (added, changed, fixed, etc.)
# 2. Enter description of the change
```

This creates a file in `.changes/unreleased/` that will be included in the next release.

### Changelog Entry Types

- **✨ Added** - New features (minor version bump)
- **🔄 Changed** - Changes to existing functionality (minor version bump)
- **💥 Breaking Changes** - Breaking API changes (major version bump)
- **⚠️ Deprecated** - Features marked for removal (minor version bump)
- **🗑️ Removed** - Removed features (minor version bump)
- **🐛 Fixed** - Bug fixes (patch version bump)
- **🔒 Security** - Security fixes (patch version bump)
- **⚡ Performance** - Performance improvements (patch version bump)
- **📚 Documentation** - Documentation only changes (patch version bump)

## Preparing a Release

### Automated Process (Recommended)

Use the GitHub Actions workflow to prepare a release:

1. **Go to Actions** → **"prepare-release"** workflow
2. **Click "Run workflow"**
3. **Enter version** (e.g., `1.0.0`)
4. **Select pre-release type** (or "none" for stable):
   - `none` - Stable release (e.g., `v1.0.0`)
   - `alpha` - Alpha release (e.g., `v1.0.0-alpha.1`)
   - `beta` - Beta release (e.g., `v1.0.0-beta.1`)
   - `rc` - Release candidate (e.g., `v1.0.0-rc.1`)

The workflow will:
- Batch all unreleased changelog entries
- Update `CHANGELOG.md`
- Create a pull request with the changes
- Auto-increment pre-release numbers if needed

### Manual Process

If you prefer to prepare releases manually:

```bash
# 1. Batch unreleased changes for a new version
changie batch patch  # For patch release (0.0.x)
changie batch minor  # For minor release (0.x.0)
changie batch major  # For major release (x.0.0)

# 2. Merge batched changes into CHANGELOG.md
changie merge

# 3. Commit and push
git add .changes/ CHANGELOG.md
git commit -m "chore: prepare release v1.0.0"
git push origin main
```

## Publishing a Release

### Automated Process (Default)

Once the prepare-release PR is merged to `main`:

1. **GitHub Actions** automatically:
   - Detects the new version from changelog
   - Creates and pushes a git tag (e.g., `v1.0.0`)
   - Triggers the release workflow

2. **Release workflow** automatically:
   - Builds binaries for all platforms (Linux, macOS - amd64/arm64)
   - Creates Docker images
   - Publishes to GitHub Container Registry
   - Creates GitHub release with:
     - Pre-built binaries
     - Checksums
     - Release notes from changie
     - Docker pull instructions

### Manual Publishing

To manually create a release:

```bash
# 1. Create and push a tag
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0

# 2. GitHub Actions will automatically trigger the release
```

## Pre-release Versions

### Creating Pre-releases

Pre-releases are useful for testing before stable releases:

```bash
# Prepare an alpha release
# Use prepare-release workflow with prerelease_type: alpha
# Results in: v1.0.0-alpha.1, v1.0.0-alpha.2, etc.

# Prepare a beta release
# Use prepare-release workflow with prerelease_type: beta
# Results in: v1.0.0-beta.1, v1.0.0-beta.2, etc.

# Prepare a release candidate
# Use prepare-release workflow with prerelease_type: rc
# Results in: v1.0.0-rc.1, v1.0.0-rc.2, etc.
```

Pre-release numbers auto-increment if the same version already exists.

### Promoting Pre-releases to Stable

To promote a pre-release to stable:

1. Run prepare-release workflow with:
   - Same base version (e.g., `1.0.0`)
   - Pre-release type: `none`
2. Merge the PR
3. Stable release (e.g., `v1.0.0`) will be created

## Version Numbering

Tasklog follows [Semantic Versioning](https://semver.org/):

- **Major (X.0.0)** - Breaking changes
- **Minor (0.X.0)** - New features, backward compatible
- **Patch (0.0.X)** - Bug fixes, backward compatible

Version format: `vMAJOR.MINOR.PATCH[-PRERELEASE.NUMBER]`

Examples:
- `v1.0.0` - Stable release
- `v1.0.0-alpha.1` - Alpha pre-release
- `v1.0.0-beta.2` - Beta pre-release
- `v1.0.0-rc.1` - Release candidate

## Troubleshooting

### Changie not found

Install changie locally:

```bash
# macOS
brew install changie

# Or using Go
go install github.com/miniscruff/changie@latest
```

### Merge conflicts in CHANGELOG.md

If you encounter merge conflicts in `CHANGELOG.md`:

1. Resolve conflicts manually
2. Keep both version sections
3. Ensure proper formatting

### Release failed

If a release fails:

1. Check GitHub Actions logs for errors
2. Fix the issue
3. Delete the failed tag: `git tag -d v1.0.0 && git push origin :v1.0.0`
4. Re-run the release process

## Release Checklist

Before releasing:

- [ ] All tests passing
- [ ] Changelog entries added for all changes
- [ ] Version number follows semantic versioning
- [ ] Pre-release type is correct (if applicable)
- [ ] Review PR created by prepare-release workflow
- [ ] No breaking changes in patch/minor releases

After releasing:

- [ ] Verify release on [GitHub Releases](https://github.com/Binsabbar/tasklog/releases)
- [ ] Verify Docker image on [GHCR](https://github.com/Binsabbar/tasklog/pkgs/container/tasklog)
- [ ] Test installing binary from release
- [ ] Announce release (if applicable)

## Manual Testing Releases

To test the release process without publishing:

```bash
# Build snapshot release locally
make release-snapshot

# Binaries will be in dist/ directory
ls -la dist/
```

## Resources

- [Changie Documentation](https://changie.dev/)
- [GoReleaser Documentation](https://goreleaser.com/)
- [Semantic Versioning](https://semver.org/)
- [GitHub Actions Workflows](.github/workflows/)
