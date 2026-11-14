# Rules for AI agents and contributors

These rules apply to all work in this repository.

## Language

- Write all text in ASD-STE100 Simplified Technical English. This includes the user interface (English), documents, code comments, workflow files and release notes.
  - Use short sentences: instructions 20 words or fewer, descriptions 25 words or fewer.
  - Use the active voice and the imperative for instructions ("Select Apply.").
  - Use one instruction in each sentence.
  - Use simple, approved words: "start" (not "launch"), "select" (not "choose"), "necessary" (not "required"), "do not" (not "don't"). Do not use contractions or idioms.
  - Use the same term for the same thing each time: "location", "region", "block", "allow", "game".
- The Portuguese and Spanish translations must be equally short and clear.
- Never use emojis anywhere: code, comments, documents, user interface, commits, issues or release notes. Badges in README.md are permitted. Gitmoji shortcodes (text such as `:sparkles:`) in commit messages are permitted.

## Commits

- Start each commit message with a gitmoji shortcode, for example `:sparkles:`, `:bug:`, `:memo:`, `:wrench:`, `:construction_worker:`, `:card_file_box:`, `:bookmark:`. See https://gitmoji.dev.
- Write the message in lowercase, short and direct, on one line. Do not add a body.
  - Good: `:sparkles: add ping test`
  - Bad: `:sparkles: Added a new ping test feature that measures the latency to all locations.`
- Author and committer: `1ukah <52342023+1ukah@users.noreply.github.com>`. Never use a personal email address.
- Do not commit personal data. `data/recordings/` is in `.gitignore` because recordings contain the servers of the internet provider of the user.

## Versions and releases

- `VERSION` contains the current version (`X.Y.Z`, Semantic Versioning).
- `CHANGELOG.md` must have a section `## [X.Y.Z]` for the version in `VERSION`. CI fails without it.
- To release:
  1. Set the version in `VERSION`.
  2. Move the changes from `## [Unreleased]` to a new section `## [X.Y.Z]`.
  3. Commit: `:bookmark: release vX.Y.Z`.
  4. Make the tag `vX.Y.Z` and push it.
  5. The release workflow checks that the tag and `VERSION` are the same, builds the app and publishes the release "CoD Server Picker vX.Y.Z" with the changelog section as the description.

## GitHub Actions

- Workflow names, job names and step names use lowercase.
- Pin each action to a full commit SHA, with the version in a comment.
- Give each workflow the minimum permissions. Use `persist-credentials: false` when a job does not push.
- Do not put `${{ }}` expressions directly in `run` scripts. Pass them through `env`.

## Code

- The app must never open a handle to the game process. It must never read, write, inject into or attach to the game process. Use only `internal/game` to find `cod.exe`.
- Run `gofmt`, `go vet ./...` and `go test ./...` before each commit.
- Build with `./build.ps1`. It writes the programs to `dist/`.
- The user interface uses Fyne. Keep it simple: most users are beginners.
