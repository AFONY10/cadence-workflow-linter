# Documentation Archive

This folder is an archive for the repository's more detailed design and implementation
notes. The project now keeps only a short, high-level overview in
`Documentation/Overview.md` to reduce drift and maintenance burden.

If you need the original detailed files, retrieve them from git history (they
were replaced in the working tree to simplify the main documentation). Example:

```powershell
# show recent commits that touched Documentation/
git log -- Documentation -n 20

# restore a specific file from history (example)
git checkout <commit-sha> -- Documentation/Implementation-Documentation.md
```

Recommended workflow
--------------------
- Keep `Documentation/Overview.md` short and authoritative.
- Use the archive only for reference; do not expand or edit the archived
  files unless the team agrees to reintroduce a longer-form doc.
