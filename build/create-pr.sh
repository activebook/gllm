#!/bin/bash

##############################################################################
# create-pr.sh
# 
# Creates a pull request from develop to main branch
# Following GitHub best practices for PR creation
##############################################################################

set -e  # Exit on error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
BASE_BRANCH="main"
HEAD_BRANCH="develop"

##############################################################################
# Functions
##############################################################################

print_step() {
    echo -e "${GREEN}==>${NC} $1"
}

print_error() {
    echo -e "${RED}Error:${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}Warning:${NC} $1"
}

check_gh_cli() {
    if ! command -v gh &> /dev/null; then
        print_error "GitHub CLI (gh) is not installed"
        echo "Please install it from: https://cli.github.com/"
        exit 1
    fi
}

check_branch_exists() {
    local branch=$1
    if ! git rev-parse --verify "$branch" &> /dev/null; then
        print_error "Branch '$branch' does not exist"
        exit 1
    fi
}

ensure_branch_updated() {
    local branch=$1
    print_step "Fetching latest changes from remote..."
    git fetch origin
    
    local local_commit=$(git rev-parse "$branch")
    local remote_commit=$(git rev-parse "origin/$branch")
    
    if [ "$local_commit" != "$remote_commit" ]; then
        print_warning "Local '$branch' is not up to date with remote"
        echo "Please pull the latest changes first:"
        echo "  git checkout $branch && git pull origin $branch"
        exit 1
    fi
}

##############################################################################
# Main Script
##############################################################################

echo ""
echo "╔════════════════════════════════════════════════════╗"
echo "║     GitHub Pull Request Creator (develop→main)     ║"
echo "╚════════════════════════════════════════════════════╝"
echo ""

# Step 1: Verify GitHub CLI is installed
print_step "Checking GitHub CLI installation..."
check_gh_cli

# Step 2: Verify we're in a git repository
print_step "Verifying git repository..."
if ! git rev-parse --git-dir > /dev/null 2>&1; then
    print_error "Not in a git repository"
    exit 1
fi

# Step 3: Check if branches exist
print_step "Verifying branches exist..."
check_branch_exists "$BASE_BRANCH"
check_branch_exists "$HEAD_BRANCH"

# Step 4: Ensure branches are up to date
ensure_branch_updated "$BASE_BRANCH"
ensure_branch_updated "$HEAD_BRANCH"

# Step 5: Check for uncommitted changes
print_step "Checking for uncommitted changes..."
if [ -n "$(git status --porcelain)" ]; then
    print_error "There are uncommitted changes in the working directory"
    echo "Please commit or stash your changes before creating a PR"
    exit 1
fi

# Step 6: Check for existing PR
print_step "Checking for existing PR..."
EXISTING_PR=$(gh pr list --base "$BASE_BRANCH" --head "$HEAD_BRANCH" --json number --jq '.[0].number' 2>/dev/null || echo "")

if [ -n "$EXISTING_PR" ]; then
    print_warning "A PR already exists (#$EXISTING_PR) from $HEAD_BRANCH to $BASE_BRANCH"
    echo "View it at: $(gh pr view "$EXISTING_PR" --json url --jq '.url')"
    read -p "Do you want to continue and create a new PR anyway? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Aborted."
        exit 0
    fi
fi

# Step 7: Generate PR details
print_step "Generating PR title and description..."

# Default title with timestamp
DEFAULT_TITLE="Merge develop into main - $(date +%Y-%m-%d)"

# Generate commit summary for description
COMMIT_COUNT=$(git rev-list --count "$BASE_BRANCH..$HEAD_BRANCH")
print_step "Found $COMMIT_COUNT commit(s) to merge"

# Create a formatted description
DESCRIPTION=$(cat <<EOF
## 🚀 Deployment PR

This PR merges the latest changes from \`develop\` into \`main\`.

### 📊 Summary
- **Commits**: $COMMIT_COUNT
- **Base**: \`$BASE_BRANCH\`
- **Head**: \`$HEAD_BRANCH\`

### 📝 Recent Changes
$(git log "$BASE_BRANCH..$HEAD_BRANCH" --oneline --max-count=10 | sed 's/^/- /')

### ✅ Checklist
- All tests pass
- Code has been reviewed
- Documentation is updated (if needed)
- Ready for production deployment
EOF
)

# Step 8: Create the PR
print_step "Creating pull request..."
echo ""
echo "Title: $DEFAULT_TITLE"
echo ""

PR_URL=$(gh pr create \
    --base "$BASE_BRANCH" \
    --head "$HEAD_BRANCH" \
    --title "$DEFAULT_TITLE" \
    --body "$DESCRIPTION" \
    2>&1)

# Check if PR creation was successful
if [ $? -eq 0 ]; then
    echo ""
    echo -e "${GREEN}✓ Pull request created successfully!${NC}"
    echo ""
    echo "$PR_URL"
    echo ""
    
    # Extract PR number from URL
    PR_NUMBER=$(echo "$PR_URL" | grep -oE '[0-9]+$')
    
    if [ -n "$PR_NUMBER" ]; then
        echo "PR #$PR_NUMBER created: $HEAD_BRANCH → $BASE_BRANCH"
        echo ""
        echo "Next steps:"
        echo "  1. Review the changes in the PR"
        echo "  2. Request reviews if needed: gh pr edit $PR_NUMBER --add-reviewer <username>"
        echo "  3. Merge when ready: ./merge-pr.sh $PR_NUMBER"
    fi
else
    print_error "Failed to create pull request"
    echo "$PR_URL"
    exit 1
fi

echo ""
