# Changelog

All important changes to this project are in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/). The project uses [Semantic Versioning](https://semver.org/).

The release job copies the section of a version into the description of its GitHub release.

## [Unreleased]

## [0.1.0] - 2026-11-14

### Added

- Block and allow server locations with Windows Firewall rules for `cod.exe`.
- 38 server locations in 11 regions, with IP ranges from the official lists of Google Cloud, Microsoft Azure and Vultr.
- Block or allow a full region or a single location.
- Presets, for example "North America only", and "Block all" and "Unblock all".
- Ping test for all locations.
- Platform setup at the first start: Steam, Battle.net, Xbox app and Other.
- Automatic game detection for Steam. For other platforms, select `cod.exe` or let the app find the running game.
- Built-in server list with online updates. A weekly job makes the server list again.
- Check for new app versions.
- Settings: language, update checks, platform and game path, "Block for all programs", "Reset audit policy" and "Remove all blocks".
- Languages: English, Portuguese and Spanish.
- Server recorder to find new locations (advanced).
- Command-line version (`cspcli`).
