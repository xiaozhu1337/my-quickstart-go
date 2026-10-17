# 功能覆盖与验证记录

对照来源为旧版 Tauri 启动器中实际接入界面的 Vue/TypeScript 和 Rust 代码。重构保留操作能力，使用 MyGo 原生组件重新布局；旧版的无边框标题栏、弹层拖动、浏览器快捷键屏蔽和拖放缺陷不照搬。

## 功能对应

| 原有能力 | 新版实现 | 主要位置 |
| --- | --- | --- |
| 分类新增、重命名、删除、选择记忆；保留至少一组 | 侧栏空白处右键添加，分类右键重命名/删除；初始 Group-1 / reverse / pentest | `internal/launcher/entries.go`、`forms.go`、`view.go` |
| 当前分类卡片、图标、名称、全文提示 | 原生虚拟网格，图片失败使用名称首字符 | `internal/launcher/view.go`、`app.go` |
| 跨分类名称搜索；空查询回当前组 | trim、忽略大小写、保留组内顺序；新查询自动选中首个匹配组，仅显示所选组结果；零匹配组变灰且不可选，清空搜索回到搜索前分类 | `internal/launcher/entries.go`、`view.go` |
| Ctrl/Cmd+F、Esc、键盘启动 | 搜索默认收起，快捷键展开聚焦，Esc 清空并收起；网格与按钮键盘行为 | `internal/launcher/view.go` |
| 网格、分类、卡片的右键菜单 | MyGo 菜单；打开、所在文件夹、编辑、移除、分类操作 | `internal/launcher/view.go` |
| 多选文件添加、取消、组首插入、同组路径与参数去重 | 保留；目录可直接拖入，不递归展开 | `internal/launcher/forms.go`、`entries.go` |
| 外部文件拖入分类、卡片前、网格末尾 | 使用 MyGo 原生文件拖放事件 | `internal/launcher/view.go` |
| 卡片同组排序、跨分类移动 | 搜索为空时拖动；按最近行和左右半区插入，卡片实时让位并显示占位；松手保存、Esc 取消；跨分类放到目标组首，列表下方空白放到末尾 | `internal/launcher/drag.go`、`view.go`、`entries.go` |
| 应用名称、路径、参数、逐行环境变量编辑 | 720 DIP 响应式宽弹窗，小窗口滚动表单且保留保存按钮；空名/路径保留旧值 | `internal/launcher/forms.go` |
| 命令新增与编辑、空名称推导、保留 ID 和添加时间 | Windows 使用 cmd；macOS/Linux 使用 sh | `internal/launcher/forms.go`、`internal/platform` |
| 单击、Enter/Space 启动；启动成功后保留窗口 | 异步启动并显示失败原因 | `internal/launcher/app.go` |
| 系统应用列表、名称过滤、添加 | Windows 入口为“添加 UWP 应用”，列表保留系统枚举范围；其他平台使用本机应用枚举 | `internal/platform`、`internal/launcher/forms.go` |
| Windows .lnk 目标、参数、原快捷方式名称 | COM 解析，失败保留 .lnk 本身 | `internal/platform/shortcut_windows.go` |
| 相对路径 | 以程序目录为基准；仅影响之后的文件导入 | `internal/platform/platform.go` |
| 文件、目录、URL、打开所在文件夹、图标 | 原生系统接口；保留旧 data URL 图标 | `internal/platform` |
| 中英文、深浅主题、字体、字体大小 | 保留；扩展石墨、纸白、午夜、松林、玫瑰五种主题，点击预览即时应用并自动保存 | `internal/launcher/settings.go`、`theme.go`、`view.go` |
| 侧栏、卡片宽高、卡片字号、图标大小 | 即时调整及持久化，保留旧范围约束 | `internal/launcher/settings.go`、`internal/model/state.go` |
| 空白双击隐藏、置顶、启动时隐藏、窗口位置大小 | 保留；恢复时增加屏幕可见性检查 | `internal/launcher/app.go`、`settings.go` |
| 全局显示/隐藏热键、最小化恢复、热键失败保留旧值 | 使用 MyGo 全局快捷键接口 | `internal/launcher/app.go` |
| 托盘显示/隐藏、退出；普通 X 隐藏 | 退出先保存；无托盘和热键时拒绝隐藏 | `internal/launcher/app.go`、`cmd/quickstart/main.go` |
| 应用图标 | 统一蓝色四格标识；运行时全局/窗口 PNG，Windows 内嵌七档 ICO 资源，macOS 应用包 ICNS | `internal/branding`、`internal/launcher/app.go`、`scripts/build.py` |
| SQLite、历史列兼容、旧数据迁移 | 独立新版库，SQLite 一致性读取、事务写入、失败保护 | `internal/store` |

16 个原设置字段均保留：`language`、`theme`、`fontFamily`、`fontSize`、`sidebarWidth`、`cardWidth`、`cardHeight`、`cardFontSize`、`cardIconScale`、`dblClickBlankToHide`、`alwaysOnTop`、`hideOnStartup`、`useRelativePath`、`rememberWindowBounds`、`windowBounds`、`toggleHotkey`。

旧版没有内置开机启动管理、Everything 搜索、名称/时间排序、分类拖动排序或备份导入导出界面，本次没有增加这些功能。环境变量仍明文保存在本地 SQLite；移除条目不会删除它指向的文件。

## 平台差异

| 行为 | Windows | macOS | Linux |
| --- | --- | --- | --- |
| 原生界面、分类、搜索、编辑、拖放、持久化 | 支持 | 支持 | 支持，需 GTK 3 |
| 普通程序与参数/环境变量 | 原生 Windows 命令行 | POSIX 参数，支持 .app | POSIX 参数，支持 .desktop |
| 命令卡片 | `cmd /D /S /C` | `sh -c` | `sh -c` |
| 系统应用 | Windows 开始菜单 / AppsFolder | 本机应用枚举 | 本机桌面应用枚举 |
| .lnk / UWP | 支持 | Windows 专属，不转换 | Windows 专属，不转换 |
| 应用图标 | 标题栏/任务栏与 exe 资源 | Dock 与应用包，窗口无独立图标 | GTK 全局/窗口图标，显示由桌面决定 |
| 托盘 | 系统通知区 | 菜单栏 | 需 AppIndicator 库与桌面支持 |
| 全局快捷键 | 系统接口 | 系统接口 | X11 或 Wayland GlobalShortcuts portal |
| 置顶、窗口位置 | 系统接口 | 受 macOS 窗口管理约束 | Wayland 位置/置顶受合成器约束 |

Windows 保留原始参数引号和反斜杠，修复旧版将反斜杠当作通用转义符而破坏路径参数的问题。.lnk 的工作目录、管理员标记和显示模式不属于原版已支持的能力。跨系统使用数据时，需要自行调整目标路径、参数及命令语法。

## 已执行验证

验证日期：2026-10-17。所有启动验证均使用临时测试数据库，没有打开或改写用户的真实启动器数据库，也没有自动启动其中的条目。

- Windows：`mise exec go@1.27.1 -- go test -race ./...`、`go vet ./...` 通过；覆盖分类/搜索、编辑器、命令、保存快照、保存失败重试、退出保存和普通关闭、无召回入口时的隐藏保护。
- Windows 实际原生窗口：中文和图标渲染、200% 缩放下的主界面与设置页、即时切换纸白配色及标题栏、退出重启后保留主题通过。X 隐藏并保留进程、托盘恢复、实际选择托盘“退出”后正常结束进程通过。当前布局预览 `windows-preview.png`、`settings-preview.png` 使用 MyGo 测试器与示例数据渲染。
- 界面精简回归：移除重复标题、操作说明、主区域计数和空闲底栏；560 与 960 宽度下跨分类搜索结果数量、错误提示跨页面显示、清除后收起提示栏、返回后保留查询均通过。
- 搜索与编辑布局回归：默认隐藏搜索与顶部添加按钮；快捷键显示、聚焦及全选原查询，Esc 收起及恢复当前组，分类匹配数随查询更新，右键添加入口和应用选择框标题一致，960×620 与 560×380 的宽编辑框尺寸及保存通过。
- 搜索分类与拖拽修复回归：新查询选择首个匹配分类，分类间切换仅显示该组匹配项，全零结果时均不可选，清空恢复搜索前分类通过；560 与 960 宽度下卡片前后、行间与卡片周围空隙落点、跨行移动、让位预览、Esc 取消、跨分类移动、搜索禁用重排通过。快速释放提交已显示落点、80 条目虚拟网格滚动后预览稳定通过；Windows 200% 原生窗口拖动预览及 SQLite 实际保存顺序通过。当前 [拖动预览](drag-preview.png) 由 MyGo 无窗口测试器渲染。
- 五主题回归：主题合法值归一化、SQLite 保存及重载通过；正文、说明、按钮文字与背景的对比度均不低于 4.5:1。960×620 与 560×380 下依次切换全部主题、设置分组导航、返回主界面、点击开关标签及键盘空格切换通过。
- MyGo 无窗口测试：实际输入搜索、Esc 清空、侧栏空白处右键新增分类、右键重命名、编辑保存、设置行为开关通过；确认左下角不再显示新增分类按钮。
- 应用图标无窗口校验：嵌入 PNG 解码与透明区域检查通过；Windows x64/ARM64 exe 的图标资源 ID 1 包含 16/24/32/48/64/128/256 七档尺寸，PNG 均与条目尺寸一致；运行时 PNG 被 Win32 图标创建接口接受，图标句柄随即释放。两个 macOS 应用包的 ICNS 与 Info.plist 引用检查通过。此次图标改动没有打开应用窗口验证外观。
- 平台启动测试：在测试子进程中核对参数及环境变量；Windows .lnk 使用临时快捷方式验证；Linux .desktop 的 Exec、占位符、工作目录和环境变量通过。
- SQLite：旧列升级、旧设置和 cardSize、任意旧环境变量、事务回滚、旧库损坏保护、已有新库保护、WAL、并发 checkpoint、一致性迁移、大于 64 MiB 数据库和目标独占创建测试通过。
- Linux amd64：将 model、platform、launcher、store 测试交叉编译后，在 WSL archlinux 中实际执行通过；本次分类入口与图标改动也实际运行 branding、launcher 测试通过，界面测试使用 MyGo 无窗口测试器。
- Windows、Linux、macOS 的 amd64 和 arm64 共六种构建通过；macOS `.app` 的 Info.plist 及 PE / ELF / Mach-O 文件头检查通过。

尚未执行 macOS 实机测试或 Linux 桌面会话交互测试。当前 WSL 没有 DISPLAY / WAYLAND_DISPLAY，不能用后台逻辑测试代替 Linux 原生桌面验收。真实 UWP 应用、不同桌面的托盘/热键、IME、多种 DPI 之间动态切换、多显示器及用户自己的数据迁移仍需在相应机器上验证。macOS 产物尚未签名或公证。早期主题修改的独立审查代理因工具生成的 Git 分支名无效而未启动；当时由主会话复核代码并完成测试。
