---
title: "Agent Skills"
weight: 5
---

# Agent Skills

`gosubc` supports installing and managing **AI Agent Skills**. Skills are Markdown files (`SKILL.md`) and supporting files that teach AI coding assistants (like Cursor, Copilot, or generic agents) how to use your CLI properly.

## Managing Skills

`gosubc` includes a built-in `skill` command group to manage these agent instructions.

### Installing a Skill

You can install a skill from a local directory or a remote git repository.

```bash
# Install a local skill
gosubc skill install ./my-local-skill

# Install a skill from a GitHub repository (e.g., github.com/owner/repo)
gosubc skill install owner/repo
```

**Scopes:**
By default, skills are installed to the `user` scope (e.g., `~/.agents/skills/`). You can install a skill for a specific project by using `--scope project`, which places it in `.agents/skills/` in your current working directory.

```bash
gosubc skill install owner/repo --scope project
```

### Updating Skills

Skills retain provenance metadata, allowing you to easily update them.

```bash
# Update a specific skill
gosubc skill update skill_name

# Update all installed skills
gosubc skill update --all
```

If you have made local modifications, you can force the update:
```bash
gosubc skill update skill_name --force
```

### Listing Installed Skills

To see what skills are currently installed:

```bash
gosubc skill list
```

### Inspecting a Skill

To view detailed metadata (provenance, installation path, revision) for a specific skill:

```bash
gosubc skill inspect skill_name
```

### Removing a Skill

To safely remove an installed skill:

```bash
gosubc skill remove skill_name
```

## Security

*   **No execution:** Skills are instructional content (Markdown, text) and are never executed by `gosubc` during installation or updates.
*   **Path traversal protection:** The skill installer includes protection against malicious skills attempting to write files outside of the designated skill directory.
