# Documentation

User-facing documentation. Built with MkDocs.

## Commands

```bash
# Local preview
mkdocs serve

# Link validation (runs in CI)
lychee --config ../.lychee.toml *.md
```

## File Structure

```
docs/
├── design.md     Architecture, design decisions
├── ops.md        Operations guide, monitoring, troubleshooting
└── security.md   Threat model, security boundaries
```

## Writing Conventions

- Heading levels: H1 for title only, body starts at H2
- Code blocks: language specifier required (```go, ```python, ```bash)
- Internal links: use relative paths (`[ops](ops.md)`)
- External links: CI validates with lychee; broken links block merge

## Notes

- Changes to `security.md` require review (security impact)
- Maintain consistency with README.md (avoid duplication, cross-reference)
- Diagrams: use Mermaid or ASCII art (avoid external image dependencies)

## Review Guidelines

- All code examples must be runnable and tested
- Security documentation changes require explicit reviewer approval
- Links must be valid (CI enforced via lychee)
