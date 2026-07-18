# QQ Quote Generator

一个使用 Go、SVG 和 resvg 实现的 QQ 引用图生成服务。服务直接计算消息布局并生成 PNG 或动态 GIF，生产运行时不需要 Chromium。

## 功能

- `/png/` 和 `/gif/` 分别返回 PNG 与 GIF；
- `/png/base64/` 和 `/gif/base64/` 返回对应图片的 Base64 文本；
- 支持纯文本、图文混排、QQ 表情、自定义头像和 QQ 头像；
- 支持 HTTP、HTTPS 和 `data:image/*` 图片；
- 支持普通图片、emoji、sticker 的尺寸规则以及 GIF/APNG 动画；
- 固定使用浅色主题；
- 单张远程图片加载失败时继续生成引用图；
- 根据系统字体 family 自动选择苹方、微软雅黑或 Noto Sans CJK。

## 项目结构

```text
.
├── main.go                         # 程序入口和依赖组装
├── internal/
│   ├── server/server.go            # Gin 路由与 HTTP 响应
│   ├── quote/                      # 消息、资源、字体、布局、SVG 和渲染流程
│   └── resvg/                      # resvg C API、CGO 封装和平台静态库目录
├── scripts/
│   ├── build-resvg.ps1             # Windows amd64 原生库构建
│   └── build-resvg.sh              # Linux/macOS 原生库构建
├── .github/workflows/release.yml   # tag 自动构建与发布
└── Dockerfile
```

`internal/resvg/lib/` 是本地生成目录，不提交到 Git。首次拉取项目后，必须先构建 resvg 静态库，再执行 Go 编译。

## 字体

服务按以下顺序选择第一个已安装的字体：

1. `PingFang SC`
2. `Microsoft YaHei`
3. `Noto Sans CJK SC`

macOS 通常自带苹方，Windows 通常自带微软雅黑。Linux 和 Docker 建议安装 Noto CJK。三者均不存在时，程序会明确启动失败，不会静默换成未知字体。

## Windows amd64 构建

### 环境要求

- Go 1.25 或更高版本；
- Rust 1.87 或更高版本；
- Cargo 和 rustup；
- Git；
- MinGW-w64 GCC，并且 `gcc` 已加入 `PATH`。

检查工具：

```powershell
go version
rustc --version
cargo --version
rustup --version
git --version
gcc --version
```

### 编译 resvg

```powershell
powershell -ExecutionPolicy Bypass -File scripts/build-resvg.ps1
```

脚本会：

1. 固定拉取 resvg 官方标签 `v0.47.0`；
2. 把源码缓存到 `%TEMP%\qq-quote-resvg-0.47.0`；
3. 安装 Rust 目标 `x86_64-pc-windows-gnu`；
4. 编译 `resvg-capi` 的 release 静态库；
5. 写入 `internal/resvg/lib/windows-amd64/libresvg.a`；
6. 同步官方 `resvg.h` 到 `internal/resvg/resvg.h`。

### 编译程序

```powershell
$env:CGO_ENABLED = "1"
$env:GOOS = "windows"
$env:GOARCH = "amd64"
$env:CC = "gcc"
go build -trimpath -ldflags "-s -w" -o qq-quote-generator.exe .
```

启动：

```powershell
./qq-quote-generator.exe
```

## Linux amd64 构建

### Alpine

```bash
apk add --no-cache go rust cargo git gcc musl-dev libunwind-dev fontconfig font-noto-cjk
sh scripts/build-resvg.sh
CGO_LDFLAGS="$(cat internal/resvg/lib/linux-amd64/native-static-libs.txt)" \
  CGO_ENABLED=1 GOOS=linux GOARCH=amd64 \
  go build -trimpath -ldflags="-s -w" -o qq-quote-generator .
```

### Debian/Ubuntu

```bash
sudo apt update
sudo apt install -y golang-go rustc cargo git gcc libc6-dev fontconfig fonts-noto-cjk
sh scripts/build-resvg.sh
CGO_LDFLAGS="$(cat internal/resvg/lib/linux-amd64/native-static-libs.txt)" \
  CGO_ENABLED=1 GOOS=linux GOARCH=amd64 \
  go build -trimpath -ldflags="-s -w" -o qq-quote-generator .
```

如果启动时提示找不到字体，可用下面的命令确认字体 family：

```bash
fc-list | grep -E "Noto Sans CJK|PingFang|Microsoft YaHei"
```

## macOS 构建

### 环境要求

- Go 1.25 或更高版本；
- Rust 1.87 或更高版本；
- Git；
- Xcode Command Line Tools。

```bash
xcode-select --install
```

### Intel macOS

```bash
sh scripts/build-resvg.sh
CGO_LDFLAGS="$(cat internal/resvg/lib/darwin-amd64/native-static-libs.txt)" \
  CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 \
  go build -trimpath -ldflags="-s -w" -o qq-quote-generator .
```

### Apple Silicon

```bash
sh scripts/build-resvg.sh
CGO_LDFLAGS="$(cat internal/resvg/lib/darwin-arm64/native-static-libs.txt)" \
  CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 \
  go build -trimpath -ldflags="-s -w" -o qq-quote-generator .
```

脚本根据 `uname -m` 自动把静态库写入 `darwin-amd64` 或 `darwin-arm64` 目录，并记录 Rust 给出的原生链接参数。

## Docker 构建

Dockerfile 使用三个阶段：

1. Rust 阶段编译 resvg 0.47.0；
2. Go 阶段启用 CGO 并链接静态库；
3. Alpine 运行阶段安装 CA 证书、fontconfig、Noto CJK 字体及 GCC/unwind 运行库。

```bash
docker build -t qq-quote-generator .
docker run -d \
  --name qq-quote-generator \
  --restart unless-stopped \
  -p 8080:5000 \
  qq-quote-generator
```

首次构建需要访问 Docker Hub、GitHub 和 crates.io。

Docker 构建默认使用 `https://goproxy.cn,direct` 下载 Go 模块。如需覆盖：

```bash
docker build --build-arg GOPROXY=https://proxy.golang.org,direct -t qq-quote-generator .
```

## 配置

| 环境变量 | 默认值 | 说明 |
| --- | --- | --- |
| `PORT` | `5000` | HTTP 监听端口 |

## API

### POST `/png/`

```bash
curl -X POST http://localhost:8080/png/ \
  -H "Content-Type: application/json" \
  -d '[{"user_id":12345,"user_nickname":"张三","message":[{"type":"text","text":"Hello!"}]}]' \
  -o out.png
```

### POST `/png/base64/`

请求体与 `/png/` 相同，响应为纯 Base64 文本。

### POST `/gif/` 与 `/gif/base64/`

请求体与 `/png/` 相同。`/gif/` 返回 `image/gif`，`/gif/base64/` 返回同一格式的纯 Base64 文本。消息中的 GIF、APNG 和 QQ APNG 表情会合成为整张动态引用图；静态请求会生成单帧 GIF。

### 图文消息

```json
[
  {
    "user_id": 12345,
    "user_nickname": "张三",
    "message": [
      {"type": "text", "text": "看这张图"},
      {"type": "face", "id": "178"},
      {"type": "image", "url": "https://example.com/image.jpg"},
      {"type": "image", "kind": "emoji", "url": "data:image/png;base64,..."},
      {"type": "image", "kind": "sticker", "url": "data:image/png;base64,..."}
    ]
  }
]
```

`message` 必须是消息片段数组，QQ 表情 `id` 必须是十进制字符串。emoji 和 QQ 表情无论单独发送还是与文字混排均为 24px；PNG 路由加载 PNG，GIF 路由优先加载 APNG。sticker 最长边为 128px。普通图片和 sticker 使用 6px 圆角，emoji 和 QQ 表情不裁圆角。动态输出最长 5 秒、最多 100 帧并无限循环。

### 自定义头像

```json
[
  {
    "user_id": 0,
    "user_nickname": "匿名",
    "avatar": "data:image/png;base64,...",
    "message": [{"type": "text", "text": "支持 URL 或 data URI 头像"}]
  }
]
```

## 自动发布 Release

推送严格符合 `vX.Y.Z` 的 tag 时，GitHub Actions 会自动构建：

- Windows amd64；
- Linux amd64；
- macOS amd64；
- macOS arm64。

每个平台都会独立编译 resvg 0.47.0 和 Go 程序。四个平台全部成功后，工作流才会创建 GitHub Release，并上传 ZIP 或 `tar.gz`。压缩包包含可执行文件和 README。

发布示例：

```bash
git tag v1.0.0
git push origin v1.0.0
```

`v1.0.0-beta`、`1.0.0` 等 tag 不会通过版本校验。
