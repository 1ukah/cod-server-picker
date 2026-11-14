# Changelog

All important changes to this project are in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/). The project uses [Semantic Versioning](https://semver.org/).

The release job copies the section of a version into the description of its GitHub release.

## [Unreleased]

## [0.1.0]

### Added

- Block and allow server locations with Windows Firewall rules for `cod.exe`.
- 38 server locations in 11 regions, with IP ranges from the official lists of Google Cloud, Microsoft Azure and Vultr.
- Presets, for example "North America only".
- Ping test for all locations.
- Automatic game detection for Steam. Detection from the running game for other platforms.
- Built-in server list with online updates. A weekly job makes the server list again.
- Check for new app versions.
- Server recorder to find new locations (advanced).
- Languages: English, Portuguese and Spanish.
- Command-line version (`cspcli`).
