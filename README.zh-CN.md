[English](README.md)

# My Quickstart Go

使用 [MyGo](https://github.com/egoist/mygo) 原生 Go 组件重构的桌面快速启动器，支持 Windows、macOS 和 Linux。界面采用按快捷键展开的搜索栏、分类侧栏、卡片网格和原生弹层；不依赖 Tauri、浏览器或 WebView。

![主界面（示例数据）](docs/windows-preview.png)

支持分类增删改、跨分类名称搜索、应用/文件/目录启动、命令、参数和逐项环境变量、系统应用选择、文件拖入、卡片排序和跨分类移动。保留中英文、字体、卡片与侧栏尺寸、窗口置顶、启动时隐藏、相对路径、窗口位置记忆、全局快捷键和托盘。完整对照及平台差异见 [功能与验证记录](docs/FEATURES.md)。

界面提供 **石墨、纸白、午夜、松林、玫瑰** 五种主题，覆盖深浅配色；在设置中点击主题预览即可即时应用并自动保存，系统标题栏同步切换深浅样式。主界面以分类和应用网格组织内容；搜索栏默认收起，Ctrl/Cmd+F 显示并聚焦，Esc 清空并收起。搜索时自动选中首个匹配分类，仅显示当前分类的匹配条目；侧栏显示各分类匹配数量，零匹配分类变灰且不可选，清空搜索恢复之前的分类。提示栏仅在有消息时出现。通过网格空白处的右键菜单添加应用、文件或命令，Windows 的应用选择入口为“添加 UWP 应用”。应用编辑弹窗宽度为 720 DIP，小窗口自动收窄，表单可滚动且保存按钮保持可见。

未搜索时，拖动卡片可插入到其他卡片前后，周围卡片实时让位并显示占位；松手后保存顺序，Esc 取消。也可拖到侧栏分类，移入该组首位，见 [拖动预览](docs/drag-preview.png)。

设置采用独立页面，按 **外观、布局、行为、快捷键、关于** 分组。“关于”显示程序版本、Go 版本、运行平台，以及可执行文件实际包含的依赖库版本。常规窗口使用左侧导航，窄窗口改用顶部标签；尺寸设置提供卡片预览。除全局快捷键需要点击“应用”验证注册结果外，其他设置自动保存。

左下角只保留设置入口；在分类侧栏空白处右键选择“添加分类”，分类自身的右键菜单可重命名、移除。窗口、托盘和应用文件统一使用蓝色四格图标：Windows 标题栏与任务栏设置原生图标，exe 内嵌七档尺寸资源，macOS 应用包提供 ICNS 并设置 Dock 图标。Linux 通过 GTK 设置默认及窗口图标，显示位置由桌面环境决定。

![设置界面](docs/settings-preview.png)

搜索与编辑器预览：[搜索匹配与灰色分类](docs/search-preview.png)、[加宽应用编辑框](docs/editor-preview.png)。

五种主题见 [主题对照图](docs/themes-preview.png)。以上预览使用 MyGo 原生界面测试器与示例数据渲染。

## 直接运行

从 [最新 Release](https://github.com/xiaozhu1337/my-quickstart-go/releases/latest) 下载对应系统和架构的预编译程序，无需安装 Go：

| 平台 | x64 / Intel | ARM64 / Apple Silicon |
| --- | --- | --- |
| Windows | [ZIP](https://github.com/xiaozhu1337/my-quickstart-go/releases/latest/download/my-quickstart-go-windows-amd64.zip) | [ZIP](https://github.com/xiaozhu1337/my-quickstart-go/releases/latest/download/my-quickstart-go-windows-arm64.zip) |
| macOS | [TAR.GZ](https://github.com/xiaozhu1337/my-quickstart-go/releases/latest/download/my-quickstart-go-darwin-amd64.tar.gz) | [TAR.GZ](https://github.com/xiaozhu1337/my-quickstart-go/releases/latest/download/my-quickstart-go-darwin-arm64.tar.gz) |
| Linux | [TAR.GZ](https://github.com/xiaozhu1337/my-quickstart-go/releases/latest/download/my-quickstart-go-linux-amd64.tar.gz) | [TAR.GZ](https://github.com/xiaozhu1337/my-quickstart-go/releases/latest/download/my-quickstart-go-linux-arm64.tar.gz) |

先解压再运行：Windows 包内为 exe，macOS 包内为 `.app`，Linux 包内为可执行文件；Unix 压缩包保留执行权限。[SHA256SUMS](https://github.com/xiaozhu1337/my-quickstart-go/releases/latest/download/SHA256SUMS) 提供六个压缩包的 SHA-256 校验值。

Windows 双击 exe；macOS 通过 Finder 打开 `.app`；Linux 在桌面会话中运行二进制。Linux 需要 GTK 3，托盘需要 `libayatana-appindicator3`；Wayland 全局热键取决于桌面的 GlobalShortcuts portal。macOS 构建尚未签名或公证，需要按本机的应用安全设置放行。

普通关闭会隐藏到托盘；从托盘菜单选择“退出”才结束进程，并在退出前同步保存。没有可用托盘和全局热键时，窗口保持显示，避免失去召回入口。全局热键初始为空，可在设置中输入 `Ctrl+Alt+Space` 并应用。Ctrl/Cmd+F 展开并聚焦主搜索，Esc 清空并收起。

用户原先通过 Windows 启动目录快捷方式实现的开机启动，可将该快捷方式目标改为新版 exe；“启动时隐藏窗口”继续生效。

启动时提前读取首屏图标（最多 128 张，等待上限 500 毫秒），并在显示前准备界面。Windows 会在窗口仍隐藏时完成真实 GPU 首帧，不闪现窗口或抢焦点；系统图标在后台 COM 线程提取。之后每次唤出也先准备当前帧。macOS/Linux 预热布局和文字，GPU 呈现仍由桌面后端在窗口映射时初始化。极慢或不可用目标会保留占位图标并继续后台加载，避免阻止窗口使用。

## 数据与旧版迁移

| 平台 | 新版默认数据库 |
| --- | --- |
| Windows | `%LOCALAPPDATA%/my-quickstart-go/launcher.db` |
| macOS | `~/Library/Application Support/my-quickstart-go/launcher.db` |
| Linux | `$XDG_CONFIG_HOME/my-quickstart-go/launcher.db`，未设时为 `~/.config/my-quickstart-go/launcher.db` |

首次启动且新版默认数据库不存在时，自动查找旧版 `my-quickstart/launcher.db` 及历史 `com.cat.tauri-app/launcher.db`。Windows 查 LocalAppData / AppData，macOS 查 Application Support，Linux 查 XDG_DATA_HOME / `~/.local/share`。读取旧库使用 SQLite 只读一致性事务，完整校验后才创建新版库；不覆盖已有新版数据库，导入失败会明确提示。迁移不写旧 DB/WAL 内容，SQLite 必要的锁及 SHM 元数据可能变化。

旧库中的分类、顺序、条目 ID、参数、环境变量、图标及设置会迁移。相对路径继续相对于**新版程序所在目录**；若原来依靠相对路径，请将新版放到合适目录，或在条目编辑器中调整路径。跨系统迁移不会把 Windows 路径、UWP 或命令语法自动转换成其他系统的等价内容。

可用 `--data "数据库完整路径"` 指定独立数据库。自定义路径不自动导入旧库；该选项也便于使用数据副本验证。加载错误不会被默认数据覆盖。旧版若仅在 WebView localStorage 中保存过数据，请先运行旧版让其写入 SQLite，再启动新版。

错误异步写入程序所在目录的 `logs/launcher.log`，该目录必须可写。设置和条目保存在 SQLite 中，继续兼容旧表结构；没有新增独立配置文件。

## 开发与构建

Go 工具链通过 [mise](https://mise.jdx.dev/) 管理，版本固定为 1.27.3；MyGo 和 SQLite 依赖由 `go.mod` / `go.sum` 锁定。构建脚本仅使用 Python 标准库。

界面回归使用 [MyGo `ui.NewTester`](https://mygo.egoist.dev/docs/ui/testing)，不打开窗口，在内存中模拟输入、点击、拖动、滚动、键盘与布局；拖拽测试覆盖 100% / 200% 缩放。运行下面的 `go test` 不会弹出应用窗口。

```sh
mise install go@1.27.3
mise exec go@1.27.3 -- go run ./cmd/quickstart
mise exec go@1.27.3 -- go test ./...
mise exec go@1.27.3 -- go test -race ./...
mise exec go@1.27.3 -- go vet ./...
```

```sh
python -X utf8 "scripts/build.py" --os windows --arch amd64
python -X utf8 "scripts/build.py" --os darwin --arch arm64
python -X utf8 "scripts/build.py" --os linux --arch amd64
```

`--arch` 支持 `amd64` 和 `arm64`；macOS 输出包含 Info.plist 和图标的 `.app`。图标源文件位于 `internal/branding`；Windows 图标资源已预生成到 `cmd/quickstart/icon_windows_amd64.syso` 和 `cmd/quickstart/icon_windows_arm64.syso`，正常构建不需要资源编译器。更换 ICO 时，可用 LLVM 的 `windres` 编译 `internal/branding/icon.rc` 为 `.res`，再用 `llvm-cvtres /MACHINE:X64` 和 `/MACHINE:ARM64` 分别更新对应 `.syso`。所有平台以 `CGO_ENABLED=0` 构建，运行时使用 MyGo 的原生系统接口。Linux 动态加载桌面系统库，不能将“无 CGO 编译”理解为“不需要桌面库”。

源代码：`cmd/quickstart` 为入口，`internal/launcher` 为原生界面及交互，`internal/model` 为状态，`internal/store` 为 SQLite 和迁移，`internal/platform` 为平台启动及快捷方式，`internal/logging` 为异步日志。
