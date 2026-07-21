# AGENTS.md

本文档面向维护本仓库的开发者和自动化编程代理。修改代码前应先阅读本文档，并把这里记录的兼容性要求视为项目级约束。

## 项目定位

QQ Quote Generator 是一个将 QQ 聊天内容渲染为引用图片的 HTTP 服务。项目使用 Go 计算布局并生成 SVG，再通过 resvg 直接栅格化为 PNG；动态资源由 Go 合成为 GIF。生产运行时不得依赖 Chromium、浏览器进程或 HTML/CSS 截图方案。

项目的长期目标是：

- 保持当前 HTTP 接口和请求格式稳定；
- 在不牺牲视觉一致性的前提下，降低渲染延迟和内存占用；
- 支持纯文本、图文混排、QQ 头像、自定义头像、QQ 表情、emoji、sticker 和 GIF/APNG 动画；
- 在 Windows amd64、Linux amd64、macOS amd64 和 macOS arm64 上可构建、可发布；
- 让布局、资源加载、SVG 生成和 PNG 栅格化各自保持清晰边界，避免重新形成单文件式实现。

## 技术基线

- Go 版本以 `go.mod` 为准，当前为 Go 1.25.8；
- SVG 栅格化固定使用 resvg C API 0.47.0；
- Go 通过 CGO 链接各平台的 resvg 静态库；
- HTTP 服务使用 Gin；
- 字体测量使用 `github.com/go-text/typesetting`；
- resvg 和 Go 字体测量都从操作系统字体集合中选择字体，不把字体文件嵌入程序，也不通过项目内字体文件加载；
- 字体 family 的优先级为 `PingFang SC`、`Microsoft YaHei`、`Noto Sans CJK SC`。找不到任何候选字体时应在启动阶段明确失败。
- Unicode emoji 字体的优先级为 `Apple Color Emoji`、`Segoe UI Emoji`；Go 度量和 SVG 字形必须使用同一个 emoji family，找不到候选字体时应在启动阶段明确失败。Docker 使用 `samuelngs/apple-emoji-ttf` 当前最新 Release 的 Linux 字体，并固定 Release 标签和 SHA-256。

不要重新引入 Chromium、Playwright、Puppeteer、chromedp 或其他浏览器运行时，也不要恢复 renderer pool 或 `pool_size` 配置。

## 代码结构

```text
.
├── main.go                         程序入口，只负责依赖组装、端口配置和启动
├── internal/
│   ├── server/server.go            Gin 路由、请求绑定和 HTTP 响应
│   ├── quote/
│   │   ├── types.go                对外 JSON 数据结构
│   │   ├── message.go              消息解析、主题和头像地址规则
│   │   ├── resource.go             HTTP/data URI 图片加载与校验
│   │   ├── font.go                 系统字体发现和文本宽度测量
│   │   ├── layout.go               卡片、消息行、气泡和分段布局
│   │   ├── svg.go                  将布局序列化为 SVG
│   │   ├── animation.go            GIF/APNG 解码、时间轴、合成和 GIF 编码
│   │   └── renderer.go             完整渲染流程与超时控制
│   └── resvg/
│       ├── resvg.go                resvg C API 的 CGO 封装和 PNG 编码
│       ├── resvg.h                 与固定 resvg 版本配套的头文件
│       └── lib/                    本地生成的各平台静态库，不提交 Git
├── scripts/
│   ├── build-resvg.ps1             Windows amd64 原生库构建脚本
│   └── build-resvg.sh              Linux/macOS 原生库构建脚本
├── .github/workflows/release.yml   tag 构建和 GitHub Release 发布
├── Dockerfile                      Alpine 多阶段生产镜像
└── README.md                       用户使用、构建和发布说明
```

新增代码应放入职责最接近的包。根目录继续只保留 `main.go` 作为程序入口，不要把业务逻辑移回根包。除非任务明确需要，不要恢复已清理的 `legacy`、`visual-regression`、`testdata`、`native` 或 `docs` 目录。

## 渲染链路

一次请求按以下顺序处理：

1. `internal/server` 将 JSON 绑定为 `[]quote.Message`；
2. `Renderer` 解析字符串消息或消息分段；
3. `ResourceLoader` 加载头像和消息图片，读取尺寸并转换为 data URI；
4. `FontManager` 使用选中的系统字体测量文本；
5. `LayoutEngine` 计算卡片、消息行、气泡、文本和图片的逻辑尺寸；
6. `svg.go` 将布局序列化为完整 SVG；
7. `internal/resvg` 使用 resvg 栅格化，并编码为 PNG；
8. PNG 路由直接返回栅格化结果；GIF 路由在静态底图上合成动画帧并编码；Base64 路由返回对应格式的 Base64 文本。

修改其中一个阶段时，应保持相邻阶段的输入输出边界稳定。布局代码不负责网络访问，SVG 代码不重新计算业务布局，CGO 封装不处理 HTTP 请求。

## 对外接口契约

以下行为是公开契约，除非需求明确要求破坏性变更，否则必须保留：

- `POST /png/` 接收 JSON 数组，成功时返回 `image/png`；
- `POST /png/base64/` 接收同样的 JSON，成功时返回 PNG 的纯 Base64 文本；
- `POST /gif/` 和 `POST /gif/base64/` 分别返回 GIF 与 GIF 的纯 Base64 文本；
- JSON 绑定失败返回 HTTP 400 和包含 `error` 的 JSON；
- 渲染失败返回 HTTP 500 和包含 `error` 的 JSON；
- `message` 必须是由 `text`/`image`/`face` 片段组成的数组；`face` 使用扁平十进制字符串 `id`；
- `avatar` 接受 HTTP、HTTPS 或 `data:image/*`；未提供头像且 `user_id > 0` 时使用 QQ 头像接口；
- 消息图片接受 HTTP、HTTPS 或 Base64 data URI；普通图片、emoji、sticker 保持各自的尺寸规则；GIF/APNG 在 GIF 路由中保留动画；
- 单张图片加载失败时记录日志并继续渲染，不使整个请求失败；
- 单个资源上限为 16 MiB，HTTP 图片客户端超时为 5 秒，完整渲染超时为 8 秒；
- 固定使用浅色主题；
- `PORT` 是唯一运行时环境变量，默认值为 `5000`。

不要擅自更名 JSON 字段、修改路由末尾斜杠、改变响应类型，或加入会改变既有请求结果的默认参数。

## 视觉与布局约束

视觉效果是项目行为的一部分。调整布局、字体、颜色、圆角、间距、换行或图片缩放时，应同时检查短文本、长文本、多行文本、中英文混排、连续多条消息和各类图片。

需要长期保持的规则包括：

- 卡片最大逻辑宽度为 600；
- 头像为 40 × 40 的圆形裁切；
- 普通消息图片最长边不超过 200，emoji 和 QQ 表情无论单独发送还是与文字混排均为 24 × 24，sticker 最长边不超过 128；所有 QQ 表情使用相同加载规则，PNG 路由加载 PNG，GIF 路由优先加载 APNG；
- 普通消息图片和 sticker 圆角为 6，emoji 与 QQ 表情不裁圆角，卡片圆角为 12；
- 文本宽度测量和 resvg 实际使用的字体 family 必须一致；
- Unicode emoji 字形使用正文字号的 1.25 倍，字体度量和 SVG 输出必须使用相同字号；
- 所有用户文本必须经过 XML 转义后再写入 SVG；
- 图片继续使用等比例缩放，不得拉伸；
- 允许不同栅格化器之间存在极小的文字抗锯齿差异，但不接受明显的尺寸、位置、换行或颜色偏差。

布局常量集中在 `internal/quote/layout.go`。不要在 `svg.go`、HTTP 层或资源加载层复制一套隐含尺寸。

## 资源与安全约束

- 只接受 `http://`、`https://` 和 `data:image/` 图片来源；其他 scheme 或任意本地文件路径必须拒绝；
- HTTP 响应必须限制读取长度，不能先无限读取再检查大小；
- data URI 必须为 Base64 编码且媒体类型属于 `image/*`；
- 新增图片格式时，必须同时确保 Go 能读取尺寸且 resvg 能解码；
- 请求取消和渲染超时必须沿 `context.Context` 传递到网络请求；
- 日志可以记录失败来源和错误，但不得输出完整 Base64 图片内容。

## 构建与原生依赖

首次构建前，先生成当前平台的 resvg 静态库：

```powershell
# Windows amd64
powershell -ExecutionPolicy Bypass -File scripts/build-resvg.ps1
```

```bash
# Linux amd64 或当前 macOS 架构
sh scripts/build-resvg.sh
```

生成物位于 `internal/resvg/lib/<platform>/`，受 `.gitignore` 管理，不应提交。Unix 构建还会生成 `native-static-libs.txt`，Go 链接时必须把其中内容传给 `CGO_LDFLAGS`。完整平台命令以 `README.md` 为准。

维护原生依赖时要同步检查：

- `scripts/build-resvg.ps1` 中的 resvg 版本和 Windows Rust target；
- `scripts/build-resvg.sh` 中的 resvg 版本、平台映射和原生链接参数；
- `internal/resvg/resvg.h` 是否与 resvg 版本匹配；
- `internal/resvg/resvg.go` 中各平台 CGO 链接选项；
- Docker 构建阶段和运行阶段的系统库；
- Docker 中 Apple Color Emoji 的 Release 标签、下载地址和 SHA-256；
- GitHub Actions 的构建矩阵和工具链；
- README 中的环境要求及命令。

Docker 默认 `GOPROXY` 为 `https://goproxy.cn,direct`，同时允许通过 build argument 覆盖。不要把个人代理地址、私有镜像源或本机绝对路径写入仓库。

## 发布约束

`.github/workflows/release.yml` 在推送 `vX.Y.Z` 形式的 tag 时构建以下产物：

- Windows amd64 ZIP；
- Linux amd64 `tar.gz`；
- macOS amd64 `tar.gz`；
- macOS arm64 `tar.gz`。

所有平台构建成功后，发布任务使用 `softprops/action-gh-release` 创建 Release 并上传产物。修改工作流时要确保发布任务具有 `contents: write` 权限，且不能依赖工作目录中存在 `.git` 才能创建 Release。

## 修改原则

- 先读完与任务有关的包，再修改；不要根据 README 猜测实现；
- 保持 Go 风格：运行 `gofmt`，错误使用 `%w` 补充上下文，包边界按职责划分；
- 优先修改现有抽象，不为单一调用引入无必要的接口或配置；
- 保留工作区中与当前任务无关的用户改动，不执行破坏性 Git 操作；
- 不提交编译产物、下载缓存、原生静态库、临时图片或视觉对比输出；
- 行为、构建方式或发布方式变化后，同步更新 `README.md` 和本文件中受影响的长期约束；
- 注释解释设计原因和非显然约束，不复述代码表面行为。

## 验证要求

根据修改范围执行最小但充分的验证。常规代码变更至少应运行：

```bash
gofmt -w <modified-go-files>
go test ./...
go vet ./...
git diff --check
```

运行 `go test` 或 `go vet` 前，当前平台的 resvg 静态库必须已生成。涉及构建脚本、CGO、Dockerfile 或 Release 工作流时，还应检查对应平台命令、YAML 语法以及原生链接参数；只有在任务明确需要时才执行耗时的完整容器或多平台构建。

完成修改前最后确认：

- 公开路由、JSON 字段和响应格式是否保持兼容；
- 是否意外引入 Chromium 或渲染池相关依赖；
- 字体测量 family 与 resvg family 是否仍一致；
- 是否保留资源大小限制、超时和失败降级；
- 是否新增了不应提交的生成文件；
- README、构建脚本、工作流和代码中的版本/平台描述是否一致。
