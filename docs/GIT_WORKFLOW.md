# Git Workflow for GoServe

## Daily Development Workflow

### Making Changes

```powershell
# 1. Make your code changes in main.go or other files
# 2. Test your changes
go run main.go -upload -modify

# 3. Check what changed
git status
git diff

# 4. Stage the changes
git add main.go
# Or stage all changes:
git add .

# 5. Commit with descriptive message
git commit -m "Add HTTPS/TLS support with Let's Encrypt"

# 6. Push to GitHub
git push
```

## Creating a New Version Release

### Version Numbering (Semantic Versioning)
- **v1.0.0** → **v1.0.1** - Bug fixes, small tweaks
- **v1.0.0** → **v1.1.0** - New features, backwards compatible
- **v1.0.0** → **v2.0.0** - Breaking changes, major overhaul

### Release Steps

```powershell
# 1. Update version number in code
#    Change "GoServe v1.0" to "GoServe v1.1" in main.go

# 2. Update documentation
#    Add new features to README.md
#    Update DEVELOPMENT.md if needed

# 3. Stage and commit version changes
git add main.go README.md
git commit -m "Release v1.1: Add HTTPS support, improve error handling"

# 4. Create version tag
git tag -a v1.1 -m "Version 1.1 - HTTPS support and better errors"

# 5. Push everything to GitHub
git push
git push --tags

# 6. Build new executables (optional)
.\build.ps1
```

## Common Git Commands

### Checking Status
```powershell
git status                    # See what files changed
git log --oneline            # View commit history
git diff                     # See unstaged changes
git diff --staged            # See staged changes
```

### Branching (for experimental features)
```powershell
git branch feature-https     # Create new branch
git checkout feature-https   # Switch to branch
# ... make changes ...
git add .
git commit -m "Implement HTTPS"
git checkout master          # Switch back to master
git merge feature-https      # Merge feature in
```

### Viewing History
```powershell
git log --oneline --graph    # Visual commit tree
git log -p main.go          # See changes to specific file
git show v1.0               # See what's in a tag
```

### Undoing Changes
```powershell
git restore main.go          # Discard uncommitted changes
git restore --staged main.go # Unstage file
git reset HEAD~1            # Undo last commit (keep changes)
git revert <commit-hash>    # Create new commit that undoes a change
```

## Commit Message Best Practices

### Good Commit Messages
```
✓ Add HTTPS support with automatic cert renewal
✓ Fix path traversal vulnerability in file handler
✓ Improve error messages for port conflicts
✓ Update README with HTTPS configuration examples
```

### Bad Commit Messages
```
✗ Fixed stuff
✗ Update
✗ Changes
✗ WIP
```

### Format
```
<type>: <short description>

<optional longer explanation>

<optional references>
```

**Types:**
- `feat:` New feature
- `fix:` Bug fix
- `docs:` Documentation changes
- `refactor:` Code restructuring
- `test:` Add/update tests
- `chore:` Maintenance tasks

## Release Checklist

Before tagging a new version:

- [ ] All features tested and working
- [ ] README.md updated with new features
- [ ] Version number updated in code
- [ ] DEVELOPMENT.md updated if architecture changed
- [ ] No uncommitted changes (`git status` clean)
- [ ] All tests pass
- [ ] Built and tested executables

```powershell
# Quick pre-release check
git status                   # Should show "nothing to commit, working tree clean"
go build main.go            # Should compile without errors
.\main.exe -h               # Should show updated version number
```

## GitHub Integration

### View Repository Online
```powershell
gh repo view --web          # Open repo in browser
```

### View Issues and Pull Requests
```powershell
gh issue list               # List open issues
gh pr list                  # List pull requests
```

### Create a Release on GitHub
```powershell
gh release create v1.1 --title "Version 1.1" --notes "Added HTTPS support"
```

## Using GitLens in VS Code

**Inline Features:**
- Hover over any line to see commit details
- Click "Open Commit" to see full changes
- View file history from the GitLens sidebar

**Compare Changes:**
- Right-click file → "Open Changes with Previous Revision"
- View side-by-side diffs

**Commit Graph:**
- Click GitLens icon in sidebar → "Commit Graph"
- Visual representation of branches and merges

## Tips

1. **Commit often** - Small, focused commits are easier to track and revert
2. **Test before committing** - Never commit broken code to master
3. **Write good messages** - Your future self will thank you
4. **Use branches** - For experimental features or major changes
5. **Tag releases** - Makes it easy to find specific versions
6. **Push regularly** - Don't lose work if your machine dies

## Quick Reference

```powershell
# Daily workflow
git add .
git commit -m "Description of changes"
git push

# New feature branch
git checkout -b feature-name
# ... work on feature ...
git commit -am "Implement feature"
git checkout master
git merge feature-name

# New release
# ... update version in code ...
git commit -am "Release v1.x"
git tag -a v1.x -m "Version 1.x description"
git push --tags
.\build.ps1
```

## Emergency Commands

### Undo last commit but keep changes
```powershell
git reset --soft HEAD~1
```

### Discard ALL local changes (be careful!)
```powershell
git reset --hard HEAD
```

### Recover from a mistake
```powershell
git reflog                  # Find the commit you want
git reset --hard <commit>   # Go back to it
```

---

**Remember:** Git is your safety net. Commit early, commit often, and don't be afraid to experiment!
