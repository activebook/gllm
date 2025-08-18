#!/bin/bash

# Exit immediately if a command exits with a non-zero status.
set -e

# Source environment variables from .env file located in the same directory as the script
if [ -f "$(dirname "$0")/.env" ]; then
  source "$(dirname "$0")/.env"
fi

# --- Configuration ---
# Determine the absolute path of the project root, assuming the script is in a subdirectory (e.g., /build)
SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
PROJECT_ROOT=$(dirname "$SCRIPT_DIR")

# The Go file containing the version string, now using an absolute path.
VERSION_FILE="$PROJECT_ROOT/cmd/version.go"

# --- Pre-flight Checks ---
# 1. Check if GITHUB_TOKEN is set
if [ -z "$GITHUB_TOKEN" ]; then
  echo "Error: GITHUB_TOKEN environment variable is not set."
  echo "Please set it to your personal access token with 'repo' scope."
  exit 1
fi

# 2. Check if on the main branch
if [ "$(git rev-parse --abbrev-ref HEAD)" != "main" ]; then
  echo "Error: You must be on the 'main' branch to release."
  exit 1
fi

# 3. Check if the working directory is clean
if ! git diff-index --quiet HEAD --; then
  echo "Error: Working directory is not clean. Please commit or stash your changes."
  exit 1
fi

echo "All checks passed. Proceeding with release..."

# --- Release Steps ---
# 1. Extract version from the Go file
# This command uses grep to find the line with 'const version', then sed to extract the version string.
VERSION=$(grep 'const version' "$VERSION_FILE" | sed -E 's/.*"([^"]+)".*//')

if [ -z "$VERSION" ]; then
  echo "Error: Could not find version in $VERSION_FILE"
  exit 1
fi

echo "Version found: $VERSION"

# 2a. Check if the tag already exists
if git rev-parse "$VERSION" >/dev/null 2>&1; then
  echo "Error: Git tag '$VERSION' already exists. Please update the version in '$VERSION_FILE' before releasing."
  exit 1
fi

# 2. Generate changelog from commits since the last tag
LATEST_TAG=$(git describe --tags --abbrev=0 2>/dev/null || echo "")
CHANGELOG=""

if [ -n "$LATEST_TAG" ]; then
  echo "Generating changelog from commits since tag: $LATEST_TAG"
  CHANGELOG=$(git log --pretty=format:"- %s" "$LATEST_TAG"..HEAD)
else
  echo "No previous tag found. Using last 10 commits for changelog."
  CHANGELOG=$(git log --pretty=format:"- %s" -n 10)
fi

# 3. Create and push a new git tag
echo "Creating git tag $VERSION..."
# Use the generated changelog in the tag annotation.
git tag -a "$VERSION" -m "Release $VERSION" -m "$CHANGELOG"
echo "Pushing tag to origin..."
git push origin "$VERSION"

# 4. Run GoReleaser
# The --clean flag ensures any previous build artifacts are removed.
echo "Running GoReleaser..."
goreleaser release --clean

echo "Release process completed successfully for version $VERSION."
