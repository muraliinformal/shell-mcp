# Coding assistant entry point

- Read [SKILL.md](SKILL.md) first for the repository map and current behavior.
- Read only source files relevant to the current task.
- Run appropriate checks after changes: gofmt, go build ./..., and go test ./... for Go changes.
- Keep verification harmless; use temporary directories for write tests and do not restart services or modify system configuration.
- Update SKILL.md when architecture, configuration, commands, or capabilities change; update README examples when applicable.
