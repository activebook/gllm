#!/bin/bash

##############################################################################
# merge-pr.sh
# 
# Merges a pull request using merge commit strategy
# Following GitHub best practices for PR merging
##############################################################################

set -e  # Exit on error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

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

print_info() {
    echo -e "${BLUE}Info:${NC} $1"
}

check_gh_cli() {
    if ! command -v gh &> /dev/null; then
        print_error "GitHub CLI (gh) is not installed"
        echo "Please install it from: https://cli.github.com/"
        exit 1
    fi
}

show_usage() {
    cat << EOF
Usage: $0 [PR_NUMBER] [OPTIONS]

Merge a pull request using best practices.

Arguments:
    PR_NUMBER       The pull request number to merge (optional if in PR branch)

Options:
    -m, --merge     Use merge commit (default, preserves full history)
    -s, --squash    Squash commits into one
    -r, --rebase    Rebase commits onto base branch
    -d, --delete    Delete branch after merge (default: keep develop branch)
    -h, --help      Show this help message

Examples:
    $0 123                    # Merge PR #123 with merge commit
    $0 123 --squash          # Merge PR #123 with squash
    $0                        # Merge current branch's PR
    
Merge Strategies:
    Merge Commit (default): Preserves all commits + creates merge commit
                           Best for: develop→main, preserving full history
    
    Squash:                 Combines all commits into one
                           Best for: feature branches, cleaning up history
    
    Rebase:                Replays commits on top of base branch
                           Best for: linear history without merge commits

EOF
}

get_pr_info() {
    local pr_number=$1
    
    PR_INFO=$(gh pr view "$pr_number" --json number,title,state,headRefName,baseRefName,mergeable,url 2>&1)
    
    if [ $? -ne 0 ]; then
        print_error "Could not find PR #$pr_number"
        echo "$PR_INFO"
        exit 1
    fi
    
    echo "$PR_INFO"
}

check_pr_status() {
    local pr_json=$1
    
    local state=$(echo "$pr_json" | jq -r '.state')
    local mergeable=$(echo "$pr_json" | jq -r '.mergeable')
    local head_branch=$(echo "$pr_json" | jq -r '.headRefName')
    local base_branch=$(echo "$pr_json" | jq -r '.baseRefName')
    
    if [ "$state" != "OPEN" ]; then
        print_error "PR is not open (current state: $state)"
        exit 1
    fi
    
    if [ "$mergeable" = "CONFLICTING" ]; then
        print_error "PR has merge conflicts that must be resolved first"
        echo "Please resolve conflicts and try again"
        exit 1
    fi
    
    # Show branch info
    print_info "Merging: $head_branch → $base_branch"
}

##############################################################################
# Main Script
##############################################################################

echo ""
echo "╔════════════════════════════════════════════════════╗"
echo "║         GitHub Pull Request Merger                 ║"
echo "╚════════════════════════════════════════════════════╝"
echo ""

# Parse arguments
PR_NUMBER=""
MERGE_STRATEGY="merge"
DELETE_BRANCH=false

while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help)
            show_usage
            exit 0
            ;;
        -m|--merge)
            MERGE_STRATEGY="merge"
            shift
            ;;
        -s|--squash)
            MERGE_STRATEGY="squash"
            shift
            ;;
        -r|--rebase)
            MERGE_STRATEGY="rebase"
            shift
            ;;
        -d|--delete)
            DELETE_BRANCH=true
            shift
            ;;
        *)
            if [ -z "$PR_NUMBER" ] && [[ $1 =~ ^[0-9]+$ ]]; then
                PR_NUMBER=$1
            else
                print_error "Unknown option: $1"
                show_usage
                exit 1
            fi
            shift
            ;;
    esac
done

# Step 1: Verify GitHub CLI is installed
print_step "Checking GitHub CLI installation..."
check_gh_cli

# Step 2: Determine PR number if not provided
if [ -z "$PR_NUMBER" ]; then
    print_step "No PR number provided, detecting from current branch..."
    PR_NUMBER=$(gh pr view --json number --jq '.number' 2>/dev/null || echo "")
    
    if [ -z "$PR_NUMBER" ]; then
        print_error "Could not determine PR number"
        echo "Please provide a PR number or run from a branch with an associated PR"
        show_usage
        exit 1
    fi
    
    print_info "Found PR #$PR_NUMBER"
fi

# Step 3: Get PR information
print_step "Fetching PR #$PR_NUMBER information..."
PR_INFO=$(get_pr_info "$PR_NUMBER")

PR_TITLE=$(echo "$PR_INFO" | jq -r '.title')
PR_URL=$(echo "$PR_INFO" | jq -r '.url')
HEAD_BRANCH=$(echo "$PR_INFO" | jq -r '.headRefName')
BASE_BRANCH=$(echo "$PR_INFO" | jq -r '.baseRefName')

echo ""
echo "PR #$PR_NUMBER: $PR_TITLE"
echo "URL: $PR_URL"
echo ""

# Step 4: Check PR status
print_step "Validating PR status..."
check_pr_status "$PR_INFO"

# Step 5: Confirm merge strategy
echo ""
print_info "Merge strategy: $MERGE_STRATEGY"

case $MERGE_STRATEGY in
    merge)
        echo "  ↳ Will create a merge commit preserving all history"
        ;;
    squash)
        echo "  ↳ Will squash all commits into one"
        ;;
    rebase)
        echo "  ↳ Will rebase commits onto base branch"
        ;;
esac

# Special warning for develop branch
if [ "$HEAD_BRANCH" = "develop" ] && [ "$DELETE_BRANCH" = true ]; then
    print_warning "You've chosen to delete the 'develop' branch after merge!"
    echo "This is usually NOT recommended for long-lived branches."
    DELETE_BRANCH=false
    print_info "Automatically disabled branch deletion for 'develop'"
fi

# Step 6: Confirm merge
echo ""
read -p "Proceed with merge? (y/N): " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Merge cancelled."
    exit 0
fi

# Step 7: Perform the merge
print_step "Merging PR #$PR_NUMBER..."
echo ""

case $MERGE_STRATEGY in
    merge)
        gh pr merge "$PR_NUMBER" --merge --delete-branch=$DELETE_BRANCH
        ;;
    squash)
        gh pr merge "$PR_NUMBER" --squash --delete-branch=$DELETE_BRANCH
        ;;
    rebase)
        gh pr merge "$PR_NUMBER" --rebase --delete-branch=$DELETE_BRANCH
        ;;
esac

# Check merge status
if [ $? -eq 0 ]; then
    echo ""
    echo -e "${GREEN}✓ Pull request merged successfully!${NC}"
    echo ""
    echo "Details:"
    echo "  • PR #$PR_NUMBER: $PR_TITLE"
    echo "  • Strategy: $MERGE_STRATEGY"
    echo "  • Merged: $HEAD_BRANCH → $BASE_BRANCH"
    
    if [ "$DELETE_BRANCH" = true ]; then
        echo "  • Branch '$HEAD_BRANCH' was deleted"
    else
        echo "  • Branch '$HEAD_BRANCH' was kept"
    fi
    
    echo ""
    print_info "Next steps:"
    echo "  1. Pull the latest changes: git checkout $BASE_BRANCH && git pull"
    echo "  2. Verify the merge in your repository"
    
    if [ "$BASE_BRANCH" = "main" ]; then
        echo "  3. Consider creating a release tag if this was a deployment"
        echo "     git tag -a v1.0.0 -m 'Release v1.0.0' && git push origin v1.0.0"
    fi
else
    print_error "Failed to merge pull request"
    exit 1
fi

echo ""
