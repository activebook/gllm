#!/bin/bash

##############################################################################
# close-pr.sh
# 
# Closes a pull request from the current branch
# Following GitHub best practices for PR management
##############################################################################

set -e  # Exit on error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
PR_NUMBER=""
BRANCH=""
REASON=""

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
    echo -e "${BLUE}ℹ${NC} $1"
}

show_usage() {
    cat << EOF
Usage: $0 [OPTIONS]

Closes a pull request from the current branch.
Supports closing by PR number or auto-detecting from current branch.

Options:
    -n, --number <number>   PR number to close
    -r, --reason <reason>   Close reason (e.g., "duplicate", "wontfix", "superseded")
    -h, --help              Show this help message

Examples:
    $0                           # Close PR from current branch
    $0 -n 42                     # Close PR #42
    $0 -n 42 -r "duplicate"      # Close PR #42 with reason
    $0 -r "superseded by #50"    # Close current branch PR with reason
EOF
}

check_gh_cli() {
    if ! command -v gh &> /dev/null; then
        print_error "GitHub CLI (gh) is not installed"
        echo "Please install it from: https://cli.github.com/"
        exit 1
    fi
}

find_pr_for_branch() {
    local branch=$1
    local pr=$(gh pr list --head "$branch" --state open --json number --jq '.[0].number' 2>/dev/null || echo "")
    
    if [ -z "$pr" ]; then
        print_error "No open PR found for branch '$branch'"
        exit 1
    fi
    
    echo "$pr"
}

close_pr() {
    local pr_number=$1
    local reason=$2
    
    # Check if PR exists
    if ! gh pr view "$pr_number" &>/dev/null; then
        print_error "PR #$pr_number does not exist or you don't have access"
        exit 1
    fi
    
    # Get PR information
    local pr_info=$(gh pr view "$pr_number" --json number,title,state,headRefName,baseRefName,url --jq '.')
    local pr_state=$(echo "$pr_info" | jq -r '.state')
    
    # Check if PR is already closed
    if [ "$pr_state" != "OPEN" ]; then
        print_warning "PR #$pr_number is already closed"
        exit 0
    fi
    
    local pr_title=$(echo "$pr_info" | jq -r '.title')
    local head_branch=$(echo "$pr_info" | jq -r '.headRefName')
    local base_branch=$(echo "$pr_info" | jq -r '.baseRefName')
    local pr_url=$(echo "$pr_info" | jq -r '.url')
    
    # Display PR information
    echo ""
    echo "╔════════════════════════════════════════════════════╗"
    echo "║             Close Pull Request                     ║"
    echo "╚════════════════════════════════════════════════════╝"
    echo ""
    print_info "PR #$pr_number"
    echo "Title:       $pr_title"
    echo "From:        $head_branch"
    echo "To:          $base_branch"
    echo "URL:         $pr_url"
    echo ""
    
    if [ -n "$reason" ]; then
        echo "Reason:      $reason"
        echo ""
    fi
    
    # Confirmation prompt
    read -p "Are you sure you want to close PR #$pr_number? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Aborted."
        exit 0
    fi
    
    # Close the PR
    print_step "Closing PR #$pr_number..."
    
    gh pr close "$pr_number"
    
    if [ $? -eq 0 ]; then
        echo ""
        echo -e "${GREEN}✓ Pull request #$pr_number closed successfully!${NC}"
        echo ""
        
        # Optional: Clean up local branch
        read -p "Do you want to delete the local branch '$head_branch'? (y/N): " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            print_step "Deleting local branch '$head_branch'..."
            git branch -d "$head_branch" 2>/dev/null || print_warning "Could not delete branch (may have uncommitted changes)"
        fi
    else
        print_error "Failed to close pull request"
        exit 1
    fi
    
    echo ""
}

##############################################################################
# Main Script
##############################################################################

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help)
            show_usage
            exit 0
            ;;
        -n|--number)
            PR_NUMBER="$2"
            shift
            shift
            ;;
        -r|--reason)
            REASON="$2"
            shift
            shift
            ;;
        *)
            print_error "Unknown option: $1"
            show_usage
            exit 1
            ;;
    esac
done

# Verify GitHub CLI is installed
print_step "Checking GitHub CLI installation..."
check_gh_cli

# Verify we're in a git repository
print_step "Verifying git repository..."
if ! git rev-parse --git-dir > /dev/null 2>&1; then
    print_error "Not in a git repository"
    exit 1
fi

# Determine PR number
if [ -z "$PR_NUMBER" ]; then
    # Auto-detect from current branch
    print_step "Detecting PR from current branch..."
    BRANCH=$(git rev-parse --abbrev-ref HEAD)
    PR_NUMBER=$(find_pr_for_branch "$BRANCH")
    print_info "Found PR #$PR_NUMBER for branch '$BRANCH'"
fi

# Close the PR
close_pr "$PR_NUMBER" "$REASON"
