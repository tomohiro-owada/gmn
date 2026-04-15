---
name: sync-gemini-cli
description: Sync gmn with upstream Gemini CLI changes and release. Use when checking for upstream updates, syncing versions, or preparing a new release.
disable-model-invocation: true
allowed-tools: Bash, Read, Edit, Write, Grep, Glob
---

# Sync gmn with Upstream Gemini CLI

Synchronize gmn (https://github.com/tomohiro-owada/gmn) with the upstream official Gemini CLI (https://github.com/google-gemini/gemini-cli).

## Prerequisites

- Current directory must be the gmn project root: `/Users/towada/projects/gemini-mini/gmn`
- You have write access to the repository

## Process

### 1. Check Versions

Compare the current gmn version with the latest official Gemini CLI version:

```bash
cd /Users/towada/projects/gemini-mini/gmn
CURRENT_VERSION=$(git describe --tags --abbrev=0 | sed 's/v//')
echo "Current gmn version: $CURRENT_VERSION"
```

Fetch the latest official Gemini CLI version:
```bash
UPSTREAM_VERSION=$(curl -s https://api.github.com/repos/google-gemini/gemini-cli/releases/latest | grep '"tag_name"' | sed -E 's/.*"v([^"]+)".*/\1/')
echo "Upstream Gemini CLI version: $UPSTREAM_VERSION"
```

If versions match, report "Already up to date with v$CURRENT_VERSION" and exit.

### 2. Fetch Upstream Changes

If versions differ:

```bash
cd /Users/towada/projects/gemini-mini
# Clone if not exists, otherwise pull
if [ ! -d "gemini-cli-latest" ]; then
  git clone https://github.com/google-gemini/gemini-cli.git gemini-cli-latest
else
  cd gemini-cli-latest && git pull && cd ..
fi

cd gemini-cli-latest
git log --oneline --no-merges v${CURRENT_VERSION}..v${UPSTREAM_VERSION}
```

### 3. Identify Relevant Changes

Focus on commits related to:
- **Code Assist API** (`packages/core/src/code_assist/`)
- **Authentication** (OAuth, credentials)
- **API client** (request/response structures)
- **Non-interactive mode** changes
- **MCP** (Model Context Protocol)
- **Error handling**

Exclude:
- TUI/interactive features
- UI/UX changes
- Documentation-only changes
- Extension/plugin system (unless affects core API)

### 4. Apply Changes to gmn

For each relevant change:
1. Read the changed files in upstream
2. Identify corresponding files in gmn:
   - `packages/core/src/code_assist/` → `internal/api/` or `internal/config/`
   - `packages/core/src/auth/` → `internal/auth/`
   - TypeScript types → Go structs
3. Apply equivalent changes in Go
4. Update comments with reference to upstream commit

### 5. Build and Test

```bash
cd /Users/towada/projects/gemini-mini/gmn
go build -o gmn .
./gmn "test"
```

Ensure:
- Build succeeds without errors
- Basic functionality works (authentication, API calls)

### 6. Commit Changes

Create a detailed commit message:

```bash
git add .
git commit -m "Sync with official Gemini CLI v${UPSTREAM_VERSION}

Changes from v${CURRENT_VERSION}..v${UPSTREAM_VERSION}:
- [List key changes applied]
- [Reference upstream PR/commit numbers]

Upstream: https://github.com/google-gemini/gemini-cli/releases/tag/v${UPSTREAM_VERSION}

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

### 7. Tag and Release

```bash
git tag -a v${UPSTREAM_VERSION} -m "Sync with official Gemini CLI v${UPSTREAM_VERSION}"
git push origin main
git push origin v${UPSTREAM_VERSION}
```

### 8. Monitor Release

Check that GitHub Actions release workflow succeeds:

```bash
gh run list --limit 3
```

Wait for completion and verify release was created.

### 9. Update Homebrew Tap (if auto-update fails)

If GoReleaser's automatic Homebrew update fails, manually update:

```bash
cd /Users/towada/projects/gemini-mini/homebrew-tap
# Update Formula/gmn.rb with new version and checksums
git add Formula/gmn.rb
git commit -m "Update gmn to v${UPSTREAM_VERSION}"
git push origin main
```

## Output

Provide a summary:
- Upstream version checked
- Changes identified and applied
- Files modified in gmn
- Build/test results
- Release status

If no changes needed, simply report versions are in sync.

## Error Handling

- If build fails, report the error and do NOT proceed with commit/release
- If tests fail, investigate and fix before releasing
- If upstream has breaking changes, create an issue for manual review
