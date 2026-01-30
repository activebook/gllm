# GitHub Pull Request Scripts

Two scripts for managing pull requests from `develop` to `main` following GitHub best practices.

## 📋 Prerequisites

- **GitHub CLI** (`gh`) must be installed and authenticated
  ```bash
  # Install GitHub CLI
  # macOS
  brew install gh
  
  # Linux
  curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg | sudo dd of=/usr/share/keyrings/githubcli-archive-keyring.gpg
  echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" | sudo tee /etc/apt/sources.list.d/github-cli.list > /dev/null
  sudo apt update
  sudo apt install gh
  
  # Authenticate
  gh auth login
  ```

## 🚀 Usage

### Create Pull Request

```bash
./create-pr.sh
```

**What it does:**
- ✅ Verifies GitHub CLI installation
- ✅ Checks that both `develop` and `main` branches exist
- ✅ Ensures branches are up-to-date with remote
- ✅ Checks for existing PRs
- ✅ Generates descriptive PR title and body with commit summary
- ✅ Creates the PR automatically

**Features:**
- Auto-generated title with date: `Merge develop into main - 2026-01-30`
- Detailed description with commit count and recent changes
- Checklist for deployment verification
- Prevents duplicate PRs (with confirmation)

### Merge Pull Request

```bash
# Merge a specific PR (recommended)
./merge-pr.sh 123

# Merge using current branch's PR
./merge-pr.sh

# Merge with different strategies
./merge-pr.sh 123 --merge    # Default: merge commit
./merge-pr.sh 123 --squash   # Squash all commits
./merge-pr.sh 123 --rebase   # Rebase commits

# Delete branch after merge (not recommended for develop)
./merge-pr.sh 123 --delete
```

**What it does:**
- ✅ Validates PR exists and is mergeable
- ✅ Checks for merge conflicts
- ✅ Shows PR details before merging
- ✅ Supports three merge strategies
- ✅ Protects long-lived branches (won't delete `develop` by default)
- ✅ Provides clear confirmation prompts

## 🎯 Merge Strategies Explained

### 1. Merge Commit (Default) ✅ Recommended for develop→main

```bash
./merge-pr.sh 123 --merge
```

**Pros:**
- Preserves complete history of all commits
- Easy to revert entire merge with `git revert -m`
- Creates clear merge point in history
- Best for tracking when features were deployed

**Cons:**
- Can create cluttered history with many merge commits

**Best for:** Production deployments, release branches, develop→main merges

### 2. Squash and Merge

```bash
./merge-pr.sh 123 --squash
```

**Pros:**
- Clean, linear history
- Hides "WIP" commits
- One commit per feature/PR

**Cons:**
- Loses individual commit history
- Makes `git bisect` less effective for large changes
- Can't easily revert specific sub-changes

**Best for:** Feature branches, experimental work, solo projects

### 3. Rebase and Merge

```bash
./merge-pr.sh 123 --rebase
```

**Pros:**
- Linear history without merge commits
- Preserves individual commits
- Clean git graph

**Cons:**
- Can be complex with conflicts
- Rewrites commit history (new SHAs)
- Dangerous if not careful

**Best for:** Teams requiring linear history with commit details

## 📚 Best Practices (Based on Research)

### Pull Request Creation
1. **Keep PRs small** - Aim for 200-400 lines of code
2. **Write clear titles** - Descriptive and scannable
3. **Add context** - Explain what changed and why
4. **Review yourself first** - Catch obvious errors before others review
5. **Ensure branch is up-to-date** - Prevent merge conflicts

### Pull Request Review
1. **Review within 2 hours** - Respect contributor time
2. **Be specific** - Clear, actionable feedback
3. **Focus on logic** - Let automated tools handle formatting
4. **Approve or request changes** - Don't leave PRs hanging

### Merging
1. **For develop→main:** Use merge commits (preserves deployment history)
2. **For feature→develop:** Consider squash (cleaner history)
3. **For hotfixes:** Use merge commits (track urgency)
4. **Always pull after merge:** Keep local repo synchronized

### Branch Hygiene
1. **Keep main/develop protected** - Require PR reviews
2. **Delete feature branches** - After merging to develop
3. **Never delete develop** - It's a long-lived branch
4. **Tag releases** - After merging to main

## 🔧 Workflow Example

```bash
# 1. Create a PR from develop to main
./create-pr.sh

# Output: "PR #123 created: develop → main"

# 2. Get reviews, run tests, verify changes

# 3. Merge the PR when ready
./merge-pr.sh 123

# 4. Pull latest changes
git checkout main
git pull origin main

# 5. Optionally tag the release
git tag -a v1.2.0 -m "Release v1.2.0"
git push origin v1.2.0
```

## ⚙️ Configuration

Both scripts use these defaults:
- **Base branch:** `main`
- **Head branch:** `develop`
- **Merge strategy:** Merge commit (preserves history)
- **Branch deletion:** Disabled for `develop`

To customize, edit the scripts:
```bash
# In create-pr.sh
BASE_BRANCH="main"
HEAD_BRANCH="develop"

# In merge-pr.sh
MERGE_STRATEGY="merge"
DELETE_BRANCH=false
```

## 🛡️ Safety Features

- **Prevents duplicate PRs** - Checks for existing PRs first
- **Validates branch state** - Ensures branches are synced with remote
- **Confirms before merging** - Requires explicit confirmation
- **Protects develop branch** - Won't delete automatically
- **Checks for conflicts** - Validates PR is mergeable
- **Color-coded output** - Easy to spot errors/warnings

## 📖 References

These scripts follow best practices from:
- GitHub Official Documentation
- Industry-standard workflows for deploy branches
- LinearB engineering metrics research
- Code review guidelines from major tech companies

## 🐛 Troubleshooting

### "gh: command not found"
Install GitHub CLI (see Prerequisites)

### "Branch is not up to date"
```bash
git checkout develop
git pull origin develop
git checkout main  
git pull origin main
```

### "PR has merge conflicts"
Resolve conflicts first:
```bash
git checkout develop
git pull origin main
# Resolve conflicts
git push origin develop
```

### Permission denied
Make scripts executable:
```bash
chmod +x create-pr.sh merge-pr.sh
```

## 📝 License

Free to use and modify for your projects.
