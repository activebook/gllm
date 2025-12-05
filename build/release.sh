#!/bin/bash

# A robust script to automate the release process.
#
# Usage: ./build/release.sh [version]
# Example: ./build/release.sh v1.13.0
#          ./build/release.sh (auto-suggests next patch version)
#
# This script will:
# 1. Perform pre-flight checks (dependencies, git status).
# 2. Determine the target version (argument or auto-suggestion).
# 3. Validate the version (must be > latest tag).
# 4. Generate a changelog since the last tag.
# 5. Optionally run in --dry-run mode.
# 6. Prompt for final confirmation, showing the changelog.
# 7. Create and push a git tag.
# 8. Run goreleaser to publish the release.
#
# It also includes two helper modes:
# --cleanup: Interactively deletes a release to recover from a failure.
# --dry-run: Runs all checks without making any changes.

# Exit immediately if a command exits with a non-zero status.
set -e

# --- Configuration ---
# Determine the absolute path of the project root, assuming the script is in a subdirectory (e.g., /build)
SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
PROJECT_ROOT=$(dirname "$SCRIPT_DIR")

# --- Helper Functions ---

# Function to increment semantic version (patch level)
increment_version() {
  local v=$1
  if [[ $v =~ ^v([0-9]+)\.([0-9]+)\.([0-9]+)$ ]]; then
    local major="${BASH_REMATCH[1]}"
    local minor="${BASH_REMATCH[2]}"
    local patch="${BASH_REMATCH[3]}"
    echo "v$major.$minor.$((patch + 1))"
  else
    echo ""
  fi
}

# Function to compare versions
# Returns 0 if v1 < v2, 1 otherwise
version_lt() {
    local v1=${1#v}
    local v2=${2#v}
    if [ "$v1" = "$v2" ]; then
        return 1
    fi
    local IFS=.
    local i ver1=($v1) ver2=($v2)
    # fill empty fields in ver1 with zeros
    for ((i=${#ver1[@]}; i<${#ver2[@]}; i++)); do
        ver1[i]=0
    done
    for ((i=0; i<${#ver1[@]}; i++)); do
        if [[ -z ${ver2[i]} ]]; then
            # fill empty fields in ver2 with zeros
            ver2[i]=0
        fi
        if ((10#${ver1[i]} < 10#${ver2[i]})); then
            return 0
        fi
        if ((10#${ver1[i]} > 10#${ver2[i]})); then
            return 1
        fi
    done
    return 1
}

# --- Mode Selection ---

# Cleanup Mode
if [ "$1" == "--cleanup" ]; then
  read -p "Enter the version to cleanup (e.g., v1.13.0): " VERSION
  if [ -z "$VERSION" ]; then
    echo "Error: Version is required."
    exit 1
  fi

  echo "--------------------------------------------------"
  echo "🔥 Automated Cleanup for version: $VERSION"
  echo "--------------------------------------------------"
  echo "WARNING: This will permanently delete the release and tag"
  echo "         from both GitHub and your local repository."
  echo "--------------------------------------------------"

  read -p "To confirm, please type the version ('$VERSION'): " CONFIRMATION
  if [ "$CONFIRMATION" != "$VERSION" ]; then
    echo "Confirmation failed. Cleanup cancelled."
    exit 1
  fi

  echo "Confirmation accepted. Proceeding with deletion..."

  # --- GitHub API Deletion ---
  echo "Attempting to delete via GitHub API..."

  # 1. Get Owner/Repo from git remote URL
  REMOTE_URL=$(git config --get remote.origin.url)
  REPO_INFO=$(echo "$REMOTE_URL" | sed -n -E 's/.*github.com[:/]([^/]+)\/(.*)\.git/\1 \2/p')
  OWNER=$(echo "$REPO_INFO" | cut -d' ' -f1)
  REPO=$(echo "$REPO_INFO" | cut -d' ' -f2)

  if [ -z "$OWNER" ] || [ -z "$REPO" ]; then
    echo "Error: Could not parse repository owner and name from git remote 'origin'."
    exit 1
  fi
  echo "Found repository: $OWNER/$REPO"

  # 2. Get Release ID from tag using GitHub API
  echo "Fetching release ID for tag $VERSION..."
  API_URL="https://api.github.com/repos/$OWNER/$REPO/releases/tags/$VERSION"
  # Use a grep/sed pipeline to parse the release ID. This is less robust than a dedicated
  # JSON parser but avoids a dependency on python or jq.
  RELEASE_ID=$(curl -s -H "Authorization: token $GITHUB_TOKEN" -H "Accept: application/vnd.github.v3+json" "$API_URL" | grep -o '"id": *[0-9]*' | head -n 1 | sed 's/"id": *//')

  if [ -z "$RELEASE_ID" ]; then
    echo "Warning: Could not find a GitHub release matching tag '$VERSION'. It may have already been deleted."
  else
    # 3. Delete the Release via API
    echo "Deleting GitHub release with ID: $RELEASE_ID"
    DELETE_URL="https://api.github.com/repos/$OWNER/$REPO/releases/$RELEASE_ID"
    curl -s -X DELETE -H "Authorization: token $GITHUB_TOKEN" -H "Accept: application/vnd.github.v3+json" "$DELETE_URL"
    echo "GitHub release deleted."
  fi

  # 4. Delete Git Tags locally and remotely
  echo "Deleting remote git tag..."
  git push --delete origin "$VERSION" || echo "Warning: Failed to delete remote tag. It may not have existed."

  echo "Deleting local tag..."
  git tag -d "$VERSION" || echo "Warning: Failed to delete local tag. It may not have existed."

  echo "✅ Cleanup for version $VERSION complete."
  exit 0
fi

# --- Initial Setup for Release ---
DRY_RUN=false
VERSION=""

# Parse arguments
for arg in "$@"; do
  if [ "$arg" == "--dry-run" ]; then
    DRY_RUN=true
  elif [[ "$arg" == v* ]]; then
    VERSION="$arg"
  fi
done

# Source environment variables from .env file located in the same directory as the script
if [ -f "$(dirname "$0")/.env" ]; then
  source "$(dirname "$0")/.env"
fi

# --- Pre-flight Checks ---
echo "Starting pre-flight checks..."

# 1. Check for required commands
for cmd in git goreleaser curl; do
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

echo "All checks passed."

# --- Version Determination & Validation ---

# Get the latest tag
LATEST_TAG=$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
echo "Latest version: $LATEST_TAG"

if [ -z "$VERSION" ]; then
  # Auto-suggest next version
  SUGGESTED_VERSION=$(increment_version "$LATEST_TAG")
  
  if [ -z "$SUGGESTED_VERSION" ]; then
      # Fallback if increment fails (e.g. non-standard tag format)
      SUGGESTED_VERSION="v0.0.1"
  fi

  echo "No version provided."
  read -p "Enter version to release (default: $SUGGESTED_VERSION): " USER_INPUT
  VERSION="${USER_INPUT:-$SUGGESTED_VERSION}"
fi

# Validate Version
if [ "$VERSION" == "$LATEST_TAG" ]; then
    echo "Error: Version $VERSION already exists."
    exit 1
fi

# Check if VERSION < LATEST_TAG
if version_lt "$VERSION" "$LATEST_TAG"; then
    echo "Error: Proposed version $VERSION is older than the latest version $LATEST_TAG."
    exit 1
fi

echo "Target Version: $VERSION"

# --- Release Steps ---

# 3. Generate changelog from commits since the last tag
CHANGELOG=""
if [ "$LATEST_TAG" != "v0.0.0" ]; then
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

echo "✅ Release process completed successfully for version $VERSION."