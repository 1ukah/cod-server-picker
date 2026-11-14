## Summary

<!-- Tell what this pull request changes and why. Use short sentences. -->

## Type of change

- [ ] Bug fix
- [ ] New feature
- [ ] New or changed server location (`data/catalog.json`)
- [ ] Translation
- [ ] Documentation
- [ ] Build or workflow

## Related issue

<!-- For example: Closes #12. Write "None" if there is no issue. -->

## How to test

<!-- Tell the steps to test this change. -->

## Screenshots

<!-- For changes to the user interface only. Remove this section if it does not apply. -->

## Checklist

- [ ] I read [CONTRIBUTING.md](../CONTRIBUTING.md) and [AGENTS.md](../AGENTS.md).
- [ ] `gofmt -l .` shows no files, and `go vet ./...` and `go test ./...` pass.
- [ ] `./build.ps1` builds the app.
- [ ] The commit messages start with a gitmoji shortcode and use one short lowercase line.
- [ ] All new text uses Simplified Technical English. Translations sound natural to native speakers.
- [ ] There are no emojis and no personal data (email addresses, IP addresses, internet providers).
- [ ] The change does not open, read, write or attach to the game process.
- [ ] For a new location: I ran `go run ./cmd/gen-regions`, and it shows no warnings.
- [ ] For a user-facing change: I added a line to `## [Unreleased]` in `CHANGELOG.md`.
