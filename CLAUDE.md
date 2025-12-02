# Dev Tools - AI Agent Guide

**IMPORTANT**: This project uses [bd (beads)](https://github.com/steveyegge/beads) for ALL issue tracking. Do NOT use markdown TODOs, task lists, or other tracking methods.

## Quick Reference

**Check ready work:**
```bash
bd ready --json
```

**Create issue:**
```bash
bd create "Issue title" -t bug|feature|task -p 0-4 --json
```

**Update issue:**
```bash
bd update bd-42 --status in_progress --json
```

**Complete work:**
```bash
bd close bd-42 --reason "Completed" --json
```

**Link discovered work:**
```bash
bd create "Found bug" -p 1 --deps discovered-from:bd-42 --json
```

## Essential Rules

- ✅ Use bd for ALL task tracking
- ✅ Always use `--json` flag for programmatic use
- ✅ Link discovered work with `discovered-from` dependencies
- ✅ Check `bd ready` before asking "what should I work on?"
- ❌ Do NOT create markdown TODO lists
- ❌ Do NOT use external issue trackers

## MCP Server (Optional)

If MCP server is available, prefer `mcp__beads__*` functions over CLI commands for better integration.

## Detailed Workflow

For comprehensive workflow including:
- Issue types and priorities
- Complete AI agent workflow
- Dependency management
- Auto-sync details
- MCP server setup
- Best practices and examples

**The bd-tracking skill loads automatically when working with issues.**

Manual trigger: "use bd-tracking" or "show bd workflow"

See also: README.md and QUICKSTART.md for full bd documentation.
