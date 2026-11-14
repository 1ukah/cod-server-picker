# CoD Server Picker

[![release](https://img.shields.io/github/v/release/1ukah/cod-server-picker)](https://github.com/1ukah/cod-server-picker/releases/latest)
[![license](https://img.shields.io/github/license/1ukah/cod-server-picker)](LICENSE)

CoD Server Picker is a free Windows app. It lets you select the server locations where Call of Duty finds matches for you.

The app works with all games that start from the Call of Duty HQ launcher (`cod.exe`).

> This project is not affiliated with, endorsed by or connected to Activision, Infinity Ward or Microsoft. All names and trademarks belong to their owners.

## Contents

- [Purpose](#purpose)
- [How it works](#how-it-works)
- [Requirements](#requirements)
- [Download and start](#download-and-start)
- [How to use the app](#how-to-use-the-app)
- [Platforms](#platforms)
- [Server locations](#server-locations)
- [Ping test](#ping-test)
- [Server list updates](#server-list-updates)
- [Settings](#settings)
- [Advanced: record servers](#advanced-record-servers)
- [FAQ](#faq)
- [Build from the source code](#build-from-the-source-code)
- [Credits](#credits)
- [License](#license)

## Purpose

Call of Duty does not let you select a server region. The game selects a region for you. It uses your location and your ping to data centers.

If you want to play in a different region, CoD Server Picker blocks the server locations that you do not want. The game then finds matches only in the locations that you allow.

## How it works

1. When you search for a match, the game sends ping tests to data centers in many countries.
2. The game uses the results to select a region.
3. CoD Server Picker adds Windows Firewall rules. Each rule blocks the IP ranges of one server location.
4. The game cannot reach the blocked locations. The game then selects a location that you allow.

The rules block only outbound UDP traffic from `cod.exe` to the selected IP ranges. Other programs, login, store and downloads are not blocked.

## Requirements

- Windows 10 or Windows 11 (64-bit).
- Administrator rights. Windows Firewall rules need administrator rights.
- An internet connection for the ping test and for server list updates. The app also works offline with its built-in server list.

## Download and start

1. Go to the [Releases](https://github.com/1ukah/cod-server-picker/releases/latest) page.
2. Download `CoDServerPicker.exe` from the latest release.
3. Start `CoDServerPicker.exe`.
4. Windows asks for administrator rights. Select **Yes**.

The app does not need installation. You can keep the file in any folder.

If Windows SmartScreen shows a warning, read [the FAQ](#windows-smartscreen-shows-a-warning-is-the-app-safe).

## How to use the app

### First start

1. Select the platform that you use to play.
   - To change the language, select the settings button (gear icon).
2. If you use Steam, the app finds the game automatically. Continue to the main screen.
3. If you use a different platform, do one of these steps:
   - Select **Select cod.exe** and find `cod.exe` in your game folder.
   - Select **Find it automatically**. Start the game and wait in the main menu. The app finds the game.

### Block server locations

1. Optional: select **Test ping** to see your ping to each location.
2. Select a preset, for example **North America only**. Or select the locations one by one.
   - A selected check box means that the location is blocked.
   - The check box of a region blocks or allows all its locations.
3. Select **Apply**.
4. Start the game again if it is open.
5. Search for a match.

Do not apply changes during a match. If you block the server of your current match, you lose the connection.

### Remove the blocks

1. Select **Unblock all** in the presets.
2. Select **Apply**.

You can also select **Remove all blocks** in the settings (gear icon).

**Reset** does not remove blocks. **Reset** cancels the changes that you did not apply.

## Platforms

| Platform | Status |
|---|---|
| Steam | Tested. The app finds the game automatically. |
| Battle.net | Not tested. Select `cod.exe` or let the app find it from the running game. |
| Xbox app (PC Game Pass) | Not tested. See the warning below. |
| Other | Select `cod.exe` or let the app find it from the running game. |

> **Warning: Xbox app.** Support for the Xbox app is not tested. Windows protects the game folder of the Xbox app. Rules for `cod.exe` can fail in this folder. If the blocks do not work, turn on **Block for all programs** in the settings. Then the rules block the selected server IP ranges for all programs on your computer, not only for the game.

## Server locations

The app shows 11 regions with 38 locations:

| Region | Locations |
|---|---|
| US East | Virginia, New Jersey, Ohio, Atlanta, Miami |
| US Central | Chicago, Iowa, St. Louis, Dallas |
| US West | Washington State, San Francisco Bay Area, Los Angeles, Phoenix, Salt Lake City, Wyoming |
| South America | Sao Paulo, Santiago |
| Europe West | London, Paris, Belgium, Amsterdam, Madrid |
| Europe Central | Frankfurt, Zurich, Milan, Warsaw |
| Europe North | Finland, Sweden |
| Middle East | Doha, Dammam |
| Africa | Johannesburg |
| Asia | Tokyo, Seoul, Hong Kong, Singapore |
| Oceania | Sydney |

Call of Duty uses servers from Google Cloud, Microsoft Azure and Vultr. The app knows which provider regions the game uses at each location. The IP ranges of each location come from the official IP lists of these providers.

## Ping test

Select **Test ping** to measure your ping to all locations. The test takes about 10 seconds.

Most game servers do not reply to ping. The app uses these methods:

- **Microsoft Azure and Vultr:** the app measures the last network router before the server, like the Windows `tracert` tool. This router is in or near the same data center.
- **Google Cloud:** the app measures a Google test page in the same Google Cloud region (gcping.com).

The result is an estimate. Your ping in the game can be a few milliseconds different.

## Server list updates

The app has a built-in server list. When the app starts, it downloads the newest server list from this project on GitHub. If the download fails, the app uses the last list that it downloaded, or the built-in list.

An automatic job makes the server list again each week. It gets the newest official IP lists from Google Cloud, Microsoft Azure and Vultr. You do not have to do anything.

If the server list changes and you have blocks, the app updates your blocks automatically.

You can turn off the updates in the settings.

## Settings

Select the settings button (gear icon) to open the settings.

| Setting | Function |
|---|---|
| **Language** | Select English, Portuguese or Spanish. |
| **Check for server list updates** | Download the newest server list when the app starts. |
| **Check for app updates** | Look for a new version of the app when the app starts. The app shows a message and a link. It does not download or install anything. |
| **Platform and game path** | Change your platform, or find `cod.exe` again. |
| **Block for all programs** | Block the selected server IP ranges for all programs, not only for the game. Use this only if the blocks do not work, for example with the Xbox app. |
| **Reset audit policy** | Turn off the Windows connection log that the recorder uses. Use this if the app stopped during a recording. The button is available only when the log is on. |
| **Remove all blocks** | Delete all firewall rules of the app. Other firewall rules do not change. |

## Advanced: record servers

Use this function if the game finds matches in a location that the app does not show.

1. Start the game.
2. In the app, go to **Advanced** > **Record servers**.
3. Select **Start**.
4. Search for matches and play.
5. Select **Stop**.
6. The app saves a file with the servers that the game used. Servers in unknown locations show as "unknown".
7. Send the file to the project in a [GitHub issue](https://github.com/1ukah/cod-server-picker/issues). We use it to add new locations.

The recording file contains the servers of your internet provider. Remove these lines before you share the file if you do not want to show them.

The recorder uses the connection log of Windows. It does not touch the game process. When you stop the recorder, the app sets the Windows log setting back to its previous value. If the app stops during a recording, use **Reset audit policy** in the settings.

## FAQ

### Can I get banned for this app?

The risk is very low, but we cannot give a guarantee. The app does not change game files. The app does not open, read, change or attach to the game process. Ricochet anti-cheat looks for these actions. The app only adds standard Windows Firewall rules, and only Windows uses them.

Paid network tools, for example ExitLag, sell a similar "region lock" function. Activision can change its rules at any time. Use the app at your own risk.

Download the app only from the [Releases](https://github.com/1ukah/cod-server-picker/releases) page of this project. Each release has a `SHA256SUMS.txt` file. Use it to make sure that your download is correct. Copies from other websites can contain malware.

### Why does the app need administrator rights?

Windows lets only administrators add or remove firewall rules. Without administrator rights, the app cannot block locations.

### Why is my ping high? Why does the game not use the location with the lowest ping?

- **The game decides.** The app only removes locations. The game selects a location from the locations that you allow. It also uses player count, skill and party size.
- **Distance.** If you block all locations near you, the game uses a location that is far away. Your ping is then higher. The app cannot make your ping lower.
- **Your internet provider.** Your provider decides the network route to each data center. A bad route gives high ping. The app cannot change routes. Speak to your internet provider.

### The game still finds matches in a blocked location. What can I do?

1. Make sure that you selected **Apply**.
2. Close the game fully and start it again.
3. Turn off your VPN. Some VPNs send the game traffic around the Windows Firewall.
4. Some antivirus programs replace the Windows Firewall. Let Windows Defender Firewall control the firewall.
5. Make sure that the game path in the settings is correct.
6. Use **Record servers** to find unknown locations, and send us the file.

### The game shows timeouts or does not find a match

You possibly blocked too many locations. Allow more locations, or select a preset. If you block all locations, the game cannot find a match.

### Windows SmartScreen shows a warning. Is the app safe?

SmartScreen shows this warning for apps without a paid code-signing certificate. To start the app, select **More info** and then **Run anyway**.

The source code is public. You can read it, and you can build the app yourself (see [Build from the source code](#build-from-the-source-code)).

### Does the app work for Warzone and other Call of Duty games?

The app blocks servers for `cod.exe`. All games that start from the Call of Duty HQ launcher use `cod.exe`. The blocks apply to all of them. We tested only Modern Warfare 4.

### How do I remove the blocks if I deleted the app?

1. Press **Windows + R**, type `wf.msc` and press **Enter**.
2. Select **Outbound Rules**.
3. Sort by **Group**.
4. Delete all rules in the group **CoDServerPicker**.

### Does the app send my data anywhere?

No. The app downloads the server list and the latest version number from this project on GitHub. The ping test sends small test packets to the server locations and to gcping.com. The app does not send personal data.

### Why only Windows?

Call of Duty on PC runs only on Windows. The app uses Windows Firewall.

## Build from the source code

You need:

- [Go](https://go.dev/dl/) 1.26 or newer.
- A C compiler (MinGW-w64), because the user interface (Fyne) needs CGO.

Do these steps in PowerShell:

```powershell
git clone https://github.com/1ukah/cod-server-picker.git
cd cod-server-picker
.\build.ps1
```

The programs are in the `dist` folder.

The project parts:

| Folder | Content |
|---|---|
| `cmd/CoDServerPicker` | The app with the user interface. |
| `cmd/cspcli` | A command-line version for tests and advanced users. |
| `cmd/gen-regions` | Makes `regions.json` from `data/catalog.json` and the provider IP lists. |
| `data/catalog.json` | The locations and the server IPs that the game used at each location. |
| `internal/` | Firewall rules, game detection, ping test, server list and recorder. |

To add a location, see [CONTRIBUTING.md](CONTRIBUTING.md).

## Credits

This project is inspired by [CS2 Server Picker](https://github.com/FN-FAL113/cs2-server-picker) by [FN-FAL113](https://github.com/FN-FAL113). Thank you for the idea and for your work for the Counter-Strike community.

## License

This project uses the [GNU General Public License v3.0](LICENSE). You can use, change and share it. If you share a changed version, you must also share its source code under the same license.
