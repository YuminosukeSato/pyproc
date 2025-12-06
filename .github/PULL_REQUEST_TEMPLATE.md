## Description

<!-- Provide a clear and concise description of your changes -->

## Type of Change

<!-- Check all that apply -->

- [ ] Bug fix (non-breaking change which fixes an issue)
- [ ] New feature (non-breaking change which adds functionality)
- [ ] Breaking change (fix or feature that would cause existing functionality to not work as expected)
- [ ] Documentation update
- [ ] Performance improvement
- [ ] Refactoring (no functional changes)
- [ ] CI/CD or build changes

## Related Issues

<!-- Link to related issues using "Closes #123" or "Fixes #456" -->

Closes #

## Checklist

<!-- Ensure all items are checked before requesting review -->

- [ ] All tests pass locally (`go test ./...` and `cd worker/python && uv run pytest`)
- [ ] Code follows the project's style guidelines (go fmt, ruff)
- [ ] Commit messages follow [Conventional Commits](https://www.conventionalcommits.org/) format
- [ ] Documentation is updated (if applicable)
- [ ] I have self-reviewed my code
- [ ] I have added tests that prove my fix/feature works (if applicable)

## Testing

### Go Tests

<!-- Provide output or summary of Go tests -->

```bash
# Run: go test -v ./...
```

### Python Tests

<!-- Provide output or summary of Python tests -->

```bash
# Run: cd worker/python && uv run pytest -v
```

## Performance Impact

<!-- Only required for performance-related changes -->

<details>
<summary>Benchmark Results (if applicable)</summary>

```bash
# Run: cd bench && go test -bench=. -benchmem
```

</details>

## Additional Context

<!-- Any other information that reviewers should know -->

## Screenshots/Examples

<!-- If applicable, add screenshots or example code demonstrating the change -->
