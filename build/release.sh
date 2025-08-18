#!/bin/bash

# A robust script to automate the release process.
#
# This script will:
# 1. Perform pre-flight checks (dependencies, git status).
# 2. Determine the version from cmd/version.go.
# 3. Generate a changelog since the last tag.
# 4. Optionally run in --dry-run mode.
# 5. Prompt for final confirmation, showing the changelog.
# 6. Create and push a git tag.
# 7. Run goreleaser to publish the release.
#
# It also includes two helper modes:
# --cleanup: Generates commands to recover from a failed release.
# --dry-run: Runs all checks without making any changes.

# Exit immediately if a command exits with a non-zero status.
set -e

# --- Configuration ---
# Determine the absolute path of the project root, assuming the script is in a subdirectory (e.g., /build)
SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
PROJECT_ROOT=$(dirname "$SCRIPT_DIR")

# The Go file containing the version string, now using an absolute path.
VERSION_FILE="$PROJECT_ROOT/cmd/version.go"

# --- Mode Selection ---

# Cleanup Mode
if [ "$1" == "--cleanup" ]; then
  VERSION=$(grep 'const version' "$VERSION_FILE" | sed -E 's/.*"([^"]+)".*/\1/')
  if [ -z "$VERSION" ]; then
    echo "Error: Could not find version in $VERSION_FILE"
    exit 1
  fi

  echo "--------------------------------------------------"
  echo "🧹 Cleanup Helper for version: $VERSION"
  echo "--------------------------------------------------"
  echo "If a release failed, run these commands to reset:"

  if command -v gh &> /dev/null; then
    echo "\n# 1. Delete the release and tag from GitHub (recommended):"
    echo "gh release delete $VERSION --cleanup-tag"
  else
    echo "\n# 1. Delete the release draft from the GitHub UI."
    echo "# 2. Delete the remote tag:"
    echo "git push --delete origin $VERSION"
  fi

  echo "\n# 3. Delete the local tag:"
  echo "git tag -d $VERSION"
  echo "--------------------------------------------------"
  exit 0
fi

# --- Initial Setup for Release ---
DRY_RUN=false
if [ "$1" == "--dry-run" ]; then
  DRY_RUN=true
  echo "Running in --dry-run mode. No changes will be made."
fi

# Source environment variables from .env file located in the same directory as the script
if [ -f "$(dirname "$0")/.env" ]; then
  source "$(dirname "$0")/.env"
fi


# --- Pre-flight Checks ---
echo "Starting pre-flight checks..."

# 1. Check for required commands
for cmd in git goreleaser; do
  if ! command -v "$cmd" &> /dev/null; then
    echo "Error: Required command '$cmd' is not installed or not in your PATH."
    exit 1
  fi
done

# 2. Check if GITHUB_TOKEN is set
if [ -z "$GITHUB_TOKEN" ]; then
  echo "Error: GITHUB_TOKEN environment variable is not set."
  echo "Please set it in 'build/.env' or export it."
  exit 1
fi

# 3. Check if on the main branch
if [ "$(git rev-parse --abbrev-ref HEAD)" != "main" ]; then
  echo "Error: You must be on the 'main' branch to release."
  exit 1
fi

# 4. Check if the working directory is clean
if ! git diff-index --quiet HEAD --; then
  echo "Error: Working directory is not clean. Please commit or stash your changes."
  exit 1
fi

echo "All checks passed. Proceeding with release..."

# --- Release Steps ---
# 1. Extract version from the Go file
VERSION=$(grep 'const version' "$VERSION_FILE" | sed -E 's/.*"([^"]+)".*/\1/')

if [ -z "$VERSION" ]; then
  echo "Error: Could not find version in $VERSION_FILE"
  exit 1
fi

echo "Version found: $VERSION"

# 2. Check if the tag already exists
if git rev-parse "$VERSION" >/dev/null 2>&1; then
  echo "Error: Git tag '$VERSION' already exists. Please update the version in '$VERSION_FILE' before releasing."
  exit 1
fi

# 3. Generate changelog from commits since the last tag
LATEST_TAG=$(git describe --tags --abbrev=0 2>/dev/null || echo "")
CHANGELOG=""

if [ -n "$LATEST_TAG" ]; then
  echo "Generating changelog from commits since tag: $LATEST_TAG"
  CHANGELOG=$(git log --pretty=format:"- %s" "$LATEST_TAG"..HEAD)
else
  echo "No previous tag found. Using last 10 commits for changelog."
  CHANGELOG=$(git log --pretty=format:"- %s" -n 10)
fi

# --- Confirmation Step ---
echo "--------------------------------------------------"
echo "🚀 Ready to release version: $VERSION"
echo "--------------------------------------------------"
echo "Changelog to be included in the tag:"
echo -e "$CHANGELOG"
echo "--------------------------------------------------"

if [ "$DRY_RUN" = true ]; then
  echo "[DRY RUN] Would create tag '$VERSION'."
  echo "[DRY RUN] Would push tag to origin."
  echo "[DRY RUN] Would run 'goreleaser release --clean'."
  exit 0
fi

read -p "Do you want to proceed with the release? (y/n) " -n 1 -r
echo # Move to a new line
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
  echo "Release cancelled."
  exit 1
fi

# --- Execution Step ---
# 4. Create and push a new git tag
echo "Creating git tag $VERSION..."
git tag -a "$VERSION" -m "Release $VERSION" -m "$CHANGELOG"

echo "Pushing tag to origin..."
git push origin "$VERSION"

# 5. Run GoReleaser
echo "Running GoReleaser..."
goreleaser release --clean

echo "\n✅ Release process completed successfully for version $VERSION."