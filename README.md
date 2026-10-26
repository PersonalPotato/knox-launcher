# Knox Launcher

A Windows app for setting up and hosting modded Project Zomboid (Build 42) games with friends. You pick mods, world settings, characters and a start location in the launcher, press START, and it opens the game for everyone with the same setup.

The in-game half is a mod called **Knox Link** (mod id `KnoxLink`).

## Download

Get `KnoxLauncher.exe` from [Releases](../../releases). Every release is built by GitHub Actions straight from this source, and the build log is linked on the release page, so you can see exactly what went into it.

### Checking the file

Each release has a `SHA256SUMS.txt`. To check your download matches, open PowerShell where you saved it and run:

```
Get-FileHash .\KnoxLauncher.exe
```

The hash should match the one in `SHA256SUMS.txt`. You can also paste the hash (or upload the file) at [virustotal.com](https://www.virustotal.com).

Windows SmartScreen may warn about it because the exe isn't code-signed. That's normal for small unsigned projects.

## What it touches on your PC

- Reads the game folder and your Steam Workshop folder (mods, items, models, map).
- Writes server and sandbox settings to `%USERPROFILE%\Zomboid\Server`.
- Keeps its own data and log in `%APPDATA%\KnoxLauncher`.
- Talks to Steam through the `steam_api64.dll` that ships with Project Zomboid (lobbies, friends, Workshop).
- Checks GitHub for a newer release every few hours, and loads the community stories and mod packs from the Knox community server (`server/`).
- Runs a small web UI on `127.0.0.1` only, locked with a random token each launch.

## Repo layout

- `launcher/` Go source for the app. The UI is plain HTML/JS in `launcher/ui`.
- `mod/KnoxLink/` the Knox Link mod (Lua). `mod/src/` the Java source for its two small helper jars.
- `server/` the community server for shared stories and mod packs (a Cloudflare Worker). Its Steam key is a Worker secret, not in this repo.

## Building it yourself

Needs Go 1.24.

```
cd launcher
go test ./...
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-H=windowsgui" -o KnoxLauncher.exe .
```

The icon and version info come from `launcher/winres/winres.json`, compiled into `rsrc_windows_amd64.syso` with [go-winres](https://github.com/tc-hib/go-winres) (`go-winres make --arch amd64`).

On Windows PowerShell, set the env vars with `$env:GOOS="windows"` etc. first.

The mod jars need a JDK 17+ and a copy of `ZombieBuddy.jar`: `mod/src/build.sh <ZombieBuddy.jar> mod/KnoxLink/42`. They aren't kept in git. The Workshop version of Knox Link has them prebuilt.

## Third-party code

- [go-webview2](https://github.com/jchv/go-webview2), [gopher-lua](https://github.com/yuin/gopher-lua), [golang.org/x/sys](https://pkg.go.dev/golang.org/x/sys)
- [three.js](https://threejs.org) and [Preact](https://preactjs.com) + htm, in `launcher/ui/vendor` (MIT)
- Fonts in `launcher/ui/fonts` are from Google Fonts (SIL Open Font License)

Project Zomboid is made by The Indie Stone. This project isn't affiliated with them.

## License

MIT, see [LICENSE](LICENSE). Bundled third-party code keeps its own license.
