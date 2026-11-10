[简体中文](README.zh-CN.md)

# My Quickstart Go

A native desktop launcher for **Windows, macOS, and Linux**, built in Go with [MyGo](https://github.com/egoist/mygo). Organize applications, files, folders, and commands into groups and launch them with a click. The interface uses native Go UI components, without a browser or WebView.

![Main window with sample entries](docs/windows-preview.png)

## Features

- **Groups and search:** create, rename, and remove groups; press Ctrl/Cmd+F to search names across all groups. A new query selects the first matching group. The sidebar shows each group's match count and disables groups with no matches; clearing the search restores the previous group.
- **Launch entries:** open applications, files, folders, URLs, and shell commands. Edit names, paths, arguments, and per-entry environment variables. Add files through the context menu or drag them into the window, and browse installed applications using the system application picker.
- **Drag to organize:** insert cards before or after another card, with a live preview as neighboring cards make room. Release to save or press Esc to cancel. Drop a card onto a sidebar group to move it to that group. Reordering is available when search is empty.
- **Five themes:** Graphite, Paper, Midnight, Forest, and Rose. Change the theme immediately from its preview card. Choose English or Simplified Chinese, adjust fonts, and customize card and sidebar dimensions.
- **Desktop integration:** tray controls, a global show/hide shortcut, always-on-top mode, hidden startup, remembered window bounds, and paths relative to the executable. Windows supports `.lnk` resolution and UWP entries; other platforms use their own application lists and launch mechanisms.
- **Local persistence:** SQLite storage with transactional saves, legacy data migration, and protection against replacing unreadable data with an empty state.

Right-click empty space in the card grid to add entries. Right-click empty space in the sidebar to add a group, or right-click a group to rename or remove it. Settings is at the far right of the top tab bar, available from both tabs.

Settings groups **Appearance** under General; **Layout, Behavior, and Hotkey** under Launcher; **Resource import** under Resources; and **About** under Information. About shows the application version, Go version, platform, and the dependency versions embedded in the executable. Wide windows use a scrollable sidebar; narrow windows use a category selector. Changes save automatically, except the global hotkey, which needs **Apply** to validate registration. The entry editor adapts to smaller windows while keeping its save button visible.

![Settings](docs/settings-preview.png)

More previews: [themes](docs/themes-preview.png), [search](docs/search-preview.png), [entry editor](docs/editor-preview.png), and [drag reordering](docs/drag-preview.png). The previews are rendered with MyGo's headless UI tester and sample data. A detailed feature comparison and validation record is available in [Chinese](docs/FEATURES.md).

## MCP and skill library

The top tabs switch between **Launcher** and **MCP / Skills**, preserving launcher search and group state. The library has search, type filters, and a single scrolling list. Rows are spaced apart and highlight on hover; click anywhere on a row to toggle its selection. Multiple selections persist across searches, with no checkboxes or pagination.

Use **Add MCP** to save a named server with YAML fields `command`, `args`, `env`, `cwd`, or HTTP `url` and `headers`. Use **Add Skill** to import a directory containing `SKILL.md` with `name` and `description` frontmatter. Supporting scripts and references are copied into the library. Imports containing symlinks are rejected; each import is limited to 100 MiB and 10,000 paths.

Select resources, choose **codex** or **pi**, select a project directory, and click **Link to project**.

MCP names can be edited after saving. Linking a renamed resource again replaces its previously managed name in that project, while preserving other servers. Duplicate resource names remain disallowed.

Both MCP and Skill rows have an **Edit** button that does not change selection. MCP configuration comments include concrete `args`, `env`, `cwd`, and HTTP `headers` examples, including on existing entries. Skill editing modifies the library's `SKILL.md` (up to 2 MiB), validates its frontmatter, and updates its displayed name and description. Supporting files and the original import source stay intact; existing project symlinks see saved content immediately. If a skill is renamed, link it again to create the new project name; old links remain usable.

To migrate existing resources, open **Settings → Resources → Resource import → Import MCPs and skills from magpie**. The default source is `~/.config/magpie` (or `$XDG_CONFIG_HOME/magpie`); portable installations can use the install folder or its `data` folder. Import copies standard stdio/HTTP MCP definitions, notes, and complete local skill folders, including skills whose root directory is a symlink. Existing same-kind names are skipped, and invalid or unsupported entries are reported individually. SSE and servers with disabled-tool/startup-timeout overrides are reported as unsupported. Magpie's own configuration and files are not changed; agent assignments, OAuth sessions, and plugin runtime state are not migrated. Importing does not deploy resources to a project.

![MCP and skill library with sample data](docs/library-preview.png)

| Agent | MCP configuration symlink | Skill directory symlinks |
| --- | --- | --- |
| Codex | `.codex/config.toml` | `.agents/skills/<name>` |
| pi (native MCP) | `.pi/mcp.json` | `.pi/skills/<name>` |

Resources live in `library` next to the database: `library.yaml` stores resource metadata and MCP definitions, `skills` holds imported assets, and `projects` holds separate generated configurations for each project. The library uses YAML; agent configurations use their required TOML or JSON formats.

Deployment adds or updates selected entries and preserves other settings and servers. Existing regular configuration files are backed up beside the original as `.quickstart-backup-<id>` before being replaced with a symlink. Generated configurations may reformat content and omit comments; backups retain the exact original bytes. Conflicting names or foreign symlinks stop deployment, and failed writes roll back changes. Editing an MCP requires linking it again to update each intended project. Removing a library entry retains deployed configurations and imported skill files so existing links remain usable; it does not uninstall project resources.

Windows symlinks require Developer Mode or administrator privileges. Both agents require project trust before loading project MCP configuration. Pi uses its current native MCP format, not older extension adapters. Credentials are stored as plain text in the local library and generated configuration files.

The library design references [magpie](https://github.com/yetone/magpie); configuration locations follow [Codex MCP](https://developers.openai.com/codex/mcp), [Codex Skills](https://developers.openai.com/codex/skills), and [pi MCP](https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/docs/mcp.md).

## Download

Download a prebuilt package from the [latest release](https://github.com/xiaozhu1337/my-quickstart-go/releases/latest):

| Platform | x64 / Intel | ARM64 / Apple Silicon |
| --- | --- | --- |
| Windows | [ZIP](https://github.com/xiaozhu1337/my-quickstart-go/releases/latest/download/my-quickstart-go-windows-amd64.zip) | [ZIP](https://github.com/xiaozhu1337/my-quickstart-go/releases/latest/download/my-quickstart-go-windows-arm64.zip) |
| macOS | [TAR.GZ](https://github.com/xiaozhu1337/my-quickstart-go/releases/latest/download/my-quickstart-go-darwin-amd64.tar.gz) | [TAR.GZ](https://github.com/xiaozhu1337/my-quickstart-go/releases/latest/download/my-quickstart-go-darwin-arm64.tar.gz) |
| Linux | [TAR.GZ](https://github.com/xiaozhu1337/my-quickstart-go/releases/latest/download/my-quickstart-go-linux-amd64.tar.gz) | [TAR.GZ](https://github.com/xiaozhu1337/my-quickstart-go/releases/latest/download/my-quickstart-go-linux-arm64.tar.gz) |

Extract the package before running it. Windows packages contain an `.exe`; macOS packages contain an `.app`; Linux packages contain the executable. Unix archives preserve executable permissions. [SHA256SUMS](https://github.com/xiaozhu1337/my-quickstart-go/releases/latest/download/SHA256SUMS) contains checksums for all six packages. See the platform requirements below; macOS builds are unsigned.

## Build and run

The Go toolchain is managed with [mise](https://mise.jdx.dev/) and pinned to **Go 1.27.3**. Go dependencies are pinned in `go.mod` and `go.sum`. The build script uses only the Python standard library.

```sh
mise install go@1.27.3
mise exec go@1.27.3 -- go run ./cmd/quickstart
```

Build Windows x64 (the only target produced by the build script):

```sh
python -X utf8 "scripts/build.py"
```

The script always builds `windows-amd64` and overwrites `bin/windows-amd64/my-quickstart-go.exe`. The existing `--os windows --arch amd64` arguments remain accepted; other targets are rejected. Keep only `windows-amd64` under `bin`; build outputs are ignored by Git. Run the resulting `.exe` on Windows. The build uses `CGO_ENABLED=0`, while MyGo loads native system libraries at runtime.

Linux requires GTK 3. Tray support also requires `libayatana-appindicator3` and desktop support; Wayland global shortcuts depend on the desktop's GlobalShortcuts portal. Window positioning and always-on-top behavior can also be constrained by Wayland. macOS builds are not signed or notarized.

Windows embeds seven icon sizes in the executable and sets native title-bar and taskbar icons. macOS bundles an ICNS file and sets the Dock icon. Linux uses GTK default and window icons, whose presentation depends on the desktop environment. The shared icon assets are in `internal/branding`.

## Everyday use

- **Ctrl/Cmd+F:** reveal and focus the search field; select an existing query when the field is already open.
- **Esc:** clear and hide search; cancel an active card drag.
- **Click or Enter/Space:** launch an entry. Successful launches leave the launcher visible.
- **Close the window:** hide it to the tray. Use **Quit** in the tray menu to exit and flush pending changes. If neither a tray nor a global shortcut is available, the window stays visible so it can still be reached.

The global hotkey is empty by default. Set a shortcut such as `Ctrl+Alt+Space` in Settings and apply it. A failed registration preserves the previous shortcut.

Hidden startup controls the launcher's initial visibility. To start it at login on Windows, create or update a shortcut in your Windows Startup folder.

Startup preloads the initial screen's icons (up to 128 cards, with a 500 ms wait limit) and prepares content before revealing the window. On Windows, the hidden native surface also draws its first GPU frame without showing or activating the window; shell icons are extracted on background COM threads. Later reveals prepare the current frame before showing it. macOS/Linux warm content layout and text; their desktop backends still initialize GPU presentation when the window is mapped. Very slow or unavailable targets retain their placeholder while icon loading continues.

## Data and legacy migration

| Platform | Default database |
| --- | --- |
| Windows | `%LOCALAPPDATA%/my-quickstart-go/launcher.db` |
| macOS | `~/Library/Application Support/my-quickstart-go/launcher.db` |
| Linux | `$XDG_CONFIG_HOME/my-quickstart-go/launcher.db`, or `~/.config/my-quickstart-go/launcher.db` if unset |

On first launch, if the new default database does not exist, the app looks for the previous Tauri launcher's `my-quickstart/launcher.db` and historical `com.cat.tauri-app/launcher.db`. It searches LocalAppData/AppData on Windows, Application Support on macOS, and XDG_DATA_HOME or `~/.local/share` on Linux.

Migration reads a consistent SQLite snapshot, validates it, and then creates the new database. It preserves groups, order, entry IDs, arguments, environment variables, icons, and settings. An existing new database is never overwritten. Migration does not modify the old DB or WAL contents, although SQLite may update locking or shared-memory metadata.

Relative paths remain relative to the **new executable's directory**. Place the executable accordingly or edit the affected entries. Moving data between operating systems does not convert Windows paths, UWP entries, or command syntax into their equivalents on another platform.

Use `--data "path/to/launcher.db"` to select a separate database. Custom paths do not trigger automatic legacy import. If the old app has only saved data in WebView localStorage, run it once to persist the data to SQLite before migrating.

Entries and settings remain in local SQLite, including environment variable values stored as plain text. Removing an entry does not delete its target. Errors are logged asynchronously to `logs/launcher.log` next to the executable, so that directory must be writable. Logs rotate at 2 MiB and keep the current file plus two backups.

## Testing

UI tests use [MyGo `ui.NewTester`](https://mygo.egoist.dev/docs/ui/testing) to simulate input, clicks, dragging, scrolling, keyboard navigation, and layout **without opening a window**. Drag tests cover 100% and 200% scale.

```sh
mise exec go@1.27.3 -- go test ./...
mise exec go@1.27.3 -- go test -race ./...
mise exec go@1.27.3 -- go vet ./...
```

Windows tests, race checks, and static checks pass. Headless UI and backend tests have also run on Linux through WSL. All six Windows/macOS/Linux architecture combinations build successfully; Windows icon resources and macOS bundle metadata have been checked. Real macOS and Linux desktop interactions remain unverified; see the [validation record](docs/FEATURES.md) for details.

## Project layout

| Directory | Purpose |
| --- | --- |
| `cmd/quickstart` | Application entry point and generated Windows icon resources |
| `internal/launcher` | Native views, interactions, and persistence scheduling |
| `internal/library` | YAML resource storage, skill imports, and transactional agent deployment |
| `internal/libraryui` | MCP / Skills list, editors, selection, and project deployment controls |
| `internal/model` | Launcher state and normalization |
| `internal/store` | SQLite storage and legacy migration |
| `internal/platform` | Platform-specific launching and shortcut handling |
| `internal/branding` | Shared PNG, ICO, and ICNS icon assets |
| `internal/logging` | Asynchronous error logging |
| `scripts` | Build tooling |

Windows icon resource objects are checked in, so normal builds do not need a resource compiler. After replacing the ICO, compile `internal/branding/icon.rc` to a `.res` file with LLVM `windres`, then regenerate the corresponding `.syso` files with `llvm-cvtres /MACHINE:X64` and `/MACHINE:ARM64`.
