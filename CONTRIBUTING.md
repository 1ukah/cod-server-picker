# Contributing

Thank you for your help. This document tells you how to add server locations and how to change the code.

Read [AGENTS.md](AGENTS.md) before you start. It contains the rules for text, commits, releases and code. In short:

- All text follows ASD-STE100 Simplified Technical English. Use short sentences, the active voice and simple words.
- Do not use emojis.
- Commit messages start with a gitmoji shortcode and use one short lowercase line, for example `:bug: fix ping for google cloud`.

## How the server list works

There are two types of data:

| Data | Source | Who changes it |
|---|---|---|
| Locations: which cities and which provider regions the game uses | `data/catalog.json`. It contains server IPs that the game used at each location. | People. We get the IPs from recordings. |
| IP ranges of each location | The official IP lists of Google Cloud, Microsoft Azure and Vultr | The weekly job, automatically |

The generator `cmd/gen-regions` connects the two. For each server IP in the catalog, it finds the provider region that contains the IP. Then it adds all IP ranges of that provider region to the location. The result is `internal/regions/regions.json`.

A person must change `data/catalog.json` only when:

- The game starts to use a new city or a new provider.
- A provider stops a region.

The weekly job opens a GitHub issue if a server IP in the catalog is not in a provider list.

## Add a location

1. Record the servers. Use **Advanced** > **Record servers** in the app, or `cspcli record`. Search for matches for a few minutes.
2. Open the recording file. Find the UDP servers with no location.
3. The game sends ping tests to many locations. A ping-test server usually shows about 6 connections. A match server shows many more.
4. Find the city of each unknown IP. Use the provider lists or an IP lookup.
5. Add the IP to `beacons` of the correct location in `data/catalog.json`. If the city is new, add a new location to the correct region.
6. Run the generator:

   ```powershell
   go run ./cmd/gen-regions
   ```

7. Make sure that the output shows no warnings.
8. Test the ping to the new location:

   ```powershell
   go run ./cmd/cspcli ping <location-id>
   ```

9. Open a pull request with `data/catalog.json` and `internal/regions/regions.json`.

Do not add IPs of these services to the catalog. The game needs them:

- Demonware (Activision login and matchmaking, AS60229).
- Valve (Steam), Akamai, Cloudflare and TCP connections on port 443.

## Rules for code

- The app must never open a handle to the game process. It must never read, write, inject into or attach to the game process. Use the functions in `internal/game` to find `cod.exe`.
- Fill in the pull request template. It opens automatically when you make a pull request.
- Run `gofmt`, `go vet ./...` and `go test ./...` before you open a pull request.
- Keep the user interface simple. Most users are beginners.
