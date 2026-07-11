# SVG + resvg Renderer Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the production Chromium renderer with a deterministic SVG layout pipeline rendered by resvg while preserving existing request and response behavior.

**Architecture:** Keep `Renderer.Render` and `Renderer.RenderBase64` as the service boundary. Split resource loading, font measurement, geometry layout, SVG serialization, and native rasterization into focused components; retain Chromium only in an isolated visual-regression tool.

**Tech Stack:** Go 1.25+, Gin, `golang.org/x/image`, resvg 0.47.0 C API through CGO, Rust/Cargo, Chromium/rod only in the legacy comparison module.

## Global Constraints

- Production must not start or depend on Chromium.
- `/png/`, `/base64/`, `Message`, `MessageSegment`, `Renderer.Render`, and `Renderer.RenderBase64` keep their contracts.
- Canvas size and non-text geometry must match Chromium exactly; only small text-edge antialiasing differences are allowed.
- A failed remote avatar or message image must not fail the complete render.
- Remove `POOL_SIZE` from production code, Dockerfile, README, and deployment examples.
- Do not add HTTP interface tests or performance benchmarks.
- Fixed regression fixtures must not depend on the network.
- Pin resvg to 0.47.0.

## File Map

- `renderer.go`: stable renderer API and orchestration.
- `message.go`: parsing, URL validation, QQ avatar resolution, and themes.
- `resource.go`: bounded HTTP/data-URI image loading.
- `font.go`: embedded fixed font and text measurement.
- `layout.go`: quote-card geometry only.
- `svg.go`: self-contained SVG serialization.
- `resvg.go`, `resvg.h`, `native/resvg/`: CGO boundary and reproducible native build.
- `tools/visual-regression/legacy/`: unchanged baseline renderer inside a separate tools module.
- `tools/visual-regression/`: old/new renderer runner and difference report.
- `testdata/visual/`: fixed messages and local images.

---

### Task 1: Lock Existing Message Semantics

**Files:**
- Create: `message.go`
- Create: `message_test.go`
- Modify: `renderer.go`
- Modify: `renderer_test.go`

**Interfaces:**
- Produces: `parseMessageField(interface{}) ([]MessageSegment, error)`, `processMessageSegments([]MessageSegment) []processedMessageSegment`, `safeImageURL(string) string`, `themeForHour(int) Theme`.

- [ ] **Step 1: Write failing compatibility tests**

```go
func TestParseMessageFieldPreservesFormats(t *testing.T) {
	tests := []struct { raw interface{}; want []MessageSegment }{
		{"你好", []MessageSegment{{Type: "text", Text: "你好"}}},
		{nil, nil},
		{float64(42), []MessageSegment{{Type: "text", Text: "42"}}},
		{[]interface{}{map[string]interface{}{"type": "image", "kind": "emoji", "url": "data:image/png;base64,AA=="}}, []MessageSegment{{Type: "image", Kind: "emoji", URL: "data:image/png;base64,AA=="}}},
	}
	for _, tt := range tests {
		got, err := parseMessageField(tt.raw)
		if err != nil { t.Fatal(err) }
		if !reflect.DeepEqual(got, tt.want) { t.Fatalf("got %#v, want %#v", got, tt.want) }
	}
}

func TestSafeImageURLKeepsOnlySupportedSources(t *testing.T) {
	for _, tt := range []struct{ raw, want string }{{" https://example.com/a.png ", "https://example.com/a.png"}, {"data:image/png;base64,AA==", "data:image/png;base64,AA=="}, {"javascript:alert(1)", ""}} {
		if got := safeImageURL(tt.raw); got != tt.want { t.Fatalf("safeImageURL(%q) = %q", tt.raw, got) }
	}
}
```

- [ ] **Step 2: Run the focused tests and confirm RED**

Run: `go test ./... -run 'TestParseMessageFieldPreservesFormats|TestSafeImageURLKeepsOnlySupportedSources'`

Expected: FAIL because the new string-based helper contract and `Theme` do not exist.

- [ ] **Step 3: Move existing helpers and introduce explicit theme values**

```go
type Theme struct { CardBG, AvatarBG, NameColor, BubbleBG, TextColor string }

var (
	lightTheme = Theme{"#f7f8fb", "#d9dee8", "#667085", "#ffffff", "#242937"}
	darkTheme  = Theme{"#1e1e2e", "#333333", "#7c7f93", "#313244", "#cdd6f4"}
)

func themeForHour(hour int) Theme {
	if hour >= 6 && hour < 18 { return lightTheme }
	return darkTheme
}
```

Move parsing, image-kind handling, URL validation, and QQ avatar URL construction without changing semantics. Remove `html/template` URL types from processed messages.

- [ ] **Step 4: Run the full unit suite and commit**

Run: `go test ./...`

Expected: PASS.

```bash
git add message.go message_test.go renderer.go renderer_test.go
git commit -m "test: lock renderer message semantics"
```

### Task 2: Add Local Fixtures, Resource Loading, and Fixed Font Metrics

**Files:**
- Create: `resource.go`
- Create: `resource_test.go`
- Create: `font.go`
- Create: `font_test.go`
- Create: `assets/fonts/NotoSansSC-Regular.ttf`
- Create: `assets/fonts/OFL.txt`
- Create: `assets/fonts/README.md`
- Create: `testdata/visual/messages.json`
- Create: `testdata/visual/avatar.png`
- Create: `testdata/visual/wide.png`
- Create: `testdata/visual/tall.png`
- Create: `testdata/visual/transparent.png`
- Create: `testdata/visual/emoji.png`
- Create: `testdata/visual/sticker.png`
- Modify: `go.mod`
- Modify: `go.sum`

**Interfaces:**
- Produces: `ResourceLoader.Load(context.Context, string) LoadedImage`, `NewFontManager([]byte) (*FontManager, error)`, `FontManager.Measure(string, float64) float64`.

- [ ] **Step 1: Write failing loader tests**

```go
func TestResourceLoaderLoadsDataURI(t *testing.T) {
	got := NewResourceLoader(&http.Client{Timeout: time.Second}, 1<<20).Load(context.Background(), fixtureDataURI(t, 8, 4))
	if got.Missing || got.Width != 8 || got.Height != 4 { t.Fatalf("loaded image = %#v", got) }
}

func TestResourceLoaderMarksFailedRemoteImageMissing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "no", 404) }))
	defer srv.Close()
	got := NewResourceLoader(srv.Client(), 1<<20).Load(context.Background(), srv.URL)
	if !got.Missing || got.Err == nil { t.Fatalf("result = %#v", got) }
}
```

- [ ] **Step 2: Run loader tests and confirm RED**

Run: `go test ./... -run TestResourceLoader`

Expected: FAIL because loader types are undefined.

- [ ] **Step 3: Implement bounded loading**

```go
type LoadedImage struct {
	DataURI string
	Width, Height int
	Missing bool
	Err error
}

type ResourceLoader struct { client *http.Client; maxBytes int64 }
```

Use `io.LimitReader(body, maxBytes+1)`, require a 2xx status, propagate context cancellation, accept only HTTP(S) and image data URIs, validate with `image.DecodeConfig`, and return an explicit missing result without trying substitute URLs.

- [ ] **Step 4: Write failing font tests**

```go
func TestFontManagerMeasuresDeterministically(t *testing.T) {
	fm, err := NewFontManager(embeddedFont)
	if err != nil { t.Fatal(err) }
	if fm.Measure("引用测试 ABC", 15) <= fm.Measure("引用", 15) { t.Fatal("invalid advances") }
}

func TestFontManagerRejectsInvalidFont(t *testing.T) {
	if _, err := NewFontManager([]byte("bad")); err == nil { t.Fatal("expected error") }
}
```

- [ ] **Step 5: Run font tests and confirm RED**

Run: `go test ./... -run TestFontManager`

Expected: FAIL because `FontManager` is undefined.

- [ ] **Step 6: Implement embedded-font measurement**

```go
//go:embed assets/fonts/NotoSansSC-Regular.ttf
var embeddedFont []byte

type FontManager struct { font *sfnt.Font }

func NewFontManager(data []byte) (*FontManager, error) {
	f, err := sfnt.Parse(data)
	if err != nil { return nil, fmt.Errorf("parse font: %w", err) }
	return &FontManager{font: f}, nil
}
```

Measure advances and kerning at 96 DPI using `golang.org/x/image/font/sfnt`. Use the identical embedded bytes in resvg. Download the official Noto Sans SC font and OFL license, record the exact upstream URL and revision, and do not use system-font fallback.

- [ ] **Step 7: Create deterministic PNG fixtures**

Generate known RGBA patterns with a temporary Go test, save only the resulting PNG files, then delete the generator. `messages.json` covers short/long Chinese and English, newline, multiple users, mixed segments, transparency, emoji, sticker, and a missing image. Online images may be used for one manual check, but stored tests reference local fixtures only.

- [ ] **Step 8: Verify and commit**

Run: `go test ./...`

Expected: PASS.

```bash
git add resource.go resource_test.go font.go font_test.go assets/fonts testdata/visual go.mod go.sum
git commit -m "feat: add image loading and fixed font metrics"
```

### Task 3: Implement Geometry Layout

**Files:**
- Create: `layout.go`
- Create: `layout_test.go`

**Interfaces:**
- Consumes: prepared messages, loaded images, and `FontManager`.
- Produces: `LayoutEngine.Layout([]PreparedMessage, Theme) (CardLayout, error)`.

- [ ] **Step 1: Write failing geometry tests**

```go
func TestContainSizePreservesAspectRatio(t *testing.T) {
	w, h := containSize(400, 200, 200, 200)
	if w != 200 || h != 100 { t.Fatalf("got %v x %v", w, h) }
}

func TestLayoutUsesCurrentTemplateGeometry(t *testing.T) {
	card := mustLayout(t, oneTextMessage("你好"))
	row := card.Rows[0]
	if row.Avatar != (Rect{X: 12, Y: 16, W: 42, H: 42}) { t.Fatalf("avatar = %#v", row.Avatar) }
	if row.ContentX != 64 || row.Nickname.FontSize != 12 || row.Bubble.PaddingX != 12 || row.Bubble.PaddingY != 8 { t.Fatalf("row = %#v", row) }
}
```

- [ ] **Step 2: Run tests and confirm RED**

Run: `go test ./... -run 'TestContainSize|TestLayoutUsesCurrentTemplateGeometry'`

Expected: FAIL because geometry types are absent.

- [ ] **Step 3: Add exact style constants and layout types**

```go
const (
	cardMaxWidth = 600.0; cardPadX = 12.0; cardPadY = 16.0
	avatarSize = 42.0; rowGap = 10.0; rowMargin = 14.0
	nicknameSize = 12.0; nicknameMargin = 4.0
	bubblePadX = 12.0; bubblePadY = 8.0
	textSize = 15.0; textLineHeight = 24.0; segmentMargin = 4.0
)

type Rect struct { X, Y, W, H float64 }
type CardLayout struct { Width, Height float64; Theme Theme; Rows []RowLayout }
```

- [ ] **Step 4: Write failing wrapping and media tests**

```go
func TestWrapTextPreservesNewlinesAndBreaksLongTokens(t *testing.T) {
	lines := testLayoutEngine(t).wrapText("第一行\nverylongunbrokenword", 60, 15)
	if len(lines) < 3 || lines[0].Text != "第一行" { t.Fatalf("lines = %#v", lines) }
}

func TestEmojiAndStickerUseCompactBounds(t *testing.T) {
	card := mustLayout(t, oneMixedMessage())
	if got := card.Rows[0].Segments[0].Rect; got.W != 42 || got.H != 42 { t.Fatalf("emoji = %#v", got) }
	if got := card.Rows[0].Segments[1].Rect; got.W > 96 || got.H > 96 { t.Fatalf("sticker = %#v", got) }
}
```

- [ ] **Step 5: Run tests and confirm RED**

Run: `go test ./... -run 'TestWrapText|TestEmojiAndSticker'`

Expected: FAIL on missing wrapping and segment layout.

- [ ] **Step 6: Implement two-pass intrinsic sizing and final layout**

First calculate intrinsic row widths, clamp the card at 600px, then reflow text to the available bubble width. Preserve explicit newlines and greedily add measured rune clusters until the next cluster exceeds the width. Empty text retains a line box. Ordinary images use contain sizing up to 200×200, emoji exactly 42×42, sticker up to 96×96, and adjacent segments add 4px. Missing images retain their kind's box but contain no image.

- [ ] **Step 7: Add empty-card and row-spacing tests**

```go
func TestEmptyMessagesProducePaddedCard(t *testing.T) {
	card := mustLayout(t, nil)
	if card.Width != 24 || card.Height != 32 { t.Fatalf("card = %#v", card) }
}

func TestRowsHaveFourteenPixelSpacing(t *testing.T) {
	card := mustLayout(t, twoTextMessages())
	a, b := card.Rows[0].Bounds, card.Rows[1].Bounds
	if b.Y-(a.Y+a.H) != 14 { t.Fatalf("rows = %#v", card.Rows) }
}
```

- [ ] **Step 8: Verify and commit**

Run: `go test ./...`

Expected: PASS.

```bash
git add layout.go layout_test.go
git commit -m "feat: implement quote card layout"
```

### Task 4: Serialize Self-Contained SVG

**Files:**
- Create: `svg.go`
- Create: `svg_test.go`

**Interfaces:**
- Consumes: `CardLayout`.
- Produces: `SVGBuilder.Build(CardLayout) ([]byte, error)`.

- [ ] **Step 1: Write failing SVG tests**

```go
func TestSVGBuilderProducesSizedSelfContainedDocument(t *testing.T) {
	svg, err := (SVGBuilder{}).Build(mustLayout(t, messageWithDataImages()))
	if err != nil { t.Fatal(err) }
	for _, want := range []string{`<svg xmlns="http://www.w3.org/2000/svg"`, `<clipPath`, `data:image/png;base64,`} {
		if !bytes.Contains(svg, []byte(want)) { t.Fatalf("missing %q", want) }
	}
}

func TestSVGBuilderEscapesUserText(t *testing.T) {
	svg, err := (SVGBuilder{}).Build(mustLayout(t, oneTextMessage(`<script>&`)))
	if err != nil { t.Fatal(err) }
	if bytes.Contains(svg, []byte("<script>")) || !bytes.Contains(svg, []byte("&lt;script&gt;&amp;")) { t.Fatalf("unsafe SVG: %s", svg) }
}
```

- [ ] **Step 2: Run tests and confirm RED**

Run: `go test ./... -run TestSVGBuilder`

Expected: FAIL because `SVGBuilder` is undefined.

- [ ] **Step 3: Implement SVG serialization**

```go
type SVGBuilder struct{}

func (SVGBuilder) Build(card CardLayout) ([]byte, error) {
	var out bytes.Buffer
	fmt.Fprintf(&out, `<svg xmlns="http://www.w3.org/2000/svg" width="%s" height="%s" viewBox="0 0 %s %s">`, px(card.Width), px(card.Height), px(card.Width), px(card.Height))
	writeCard(&out, card)
	out.WriteString(`</svg>`)
	return out.Bytes(), nil
}
```

Implement `writeCard`, `writeRow`, `writeBubble`, `writeImage`, and `writeText` as focused serializers. `writeText` must use `xml.EscapeText`; bubbles use paths for asymmetric corners; avatars use circle clip paths; images accept only data URIs and missing resources emit no `<image>`.

- [ ] **Step 4: Test both themes, verify, and commit**

Add assertions that all five theme colors appear and no external image URL appears. Run `go test ./...`, expect PASS, then commit:

```bash
git add svg.go svg_test.go
git commit -m "feat: serialize quote layouts as svg"
```

### Task 5: Integrate resvg and Replace Production Chromium

**Files:**
- Create: `resvg.go`
- Create: `resvg.h`
- Create: `resvg_test.go`
- Create: `native/resvg/build.ps1`
- Create: `native/resvg/build.sh`
- Modify: `renderer.go`
- Modify: `main.go`
- Delete: `pool.go`
- Delete: `pool_test.go`
- Modify: `go.mod`
- Modify: `go.sum`

**Interfaces:**
- Produces: `NewResvgRasterizer([]byte) (*ResvgRasterizer, error)`, `ResvgRasterizer.Render([]byte) ([]byte, error)`, `NewRenderer() (*Renderer, error)`.

- [ ] **Step 1: Write a failing rasterizer test**

```go
func TestResvgRasterizerReturnsPNGAtSVGSize(t *testing.T) {
	r, err := NewResvgRasterizer(embeddedFont)
	if err != nil { t.Fatal(err) }
	b, err := r.Render([]byte(`<svg xmlns="http://www.w3.org/2000/svg" width="7" height="5"><rect width="7" height="5" fill="#123456"/></svg>`))
	if err != nil { t.Fatal(err) }
	cfg, err := png.DecodeConfig(bytes.NewReader(b))
	if err != nil || cfg.Width != 7 || cfg.Height != 5 { t.Fatalf("config=%#v err=%v", cfg, err) }
}
```

- [ ] **Step 2: Run it and confirm RED**

Run: `go test ./... -run TestResvgRasterizerReturnsPNGAtSVGSize`

Expected: FAIL because the adapter is absent.

- [ ] **Step 3: Add the official 0.47.0 C API build**

Build scripts fetch the official `v0.47.0` tag into a build cache, execute `cargo build --release -p resvg-capi`, copy the static library into `native/resvg/lib/<platform>`, and verify the matching header contains `RESVG_VERSION "0.47.0"`. A failed fetch/build must stop; scripts must not select another version.

- [ ] **Step 4: Implement the CGO wrapper**

```go
/*
#cgo windows LDFLAGS: -L${SRCDIR}/native/resvg/lib/windows-amd64 -lresvg -lws2_32 -luserenv -lbcrypt -lntdll
#cgo linux LDFLAGS: -L${SRCDIR}/native/resvg/lib/linux-amd64 -lresvg -ldl -lm -lpthread
#include <stdlib.h>
#include "resvg.h"
*/
import "C"

type ResvgRasterizer struct { font []byte }
```

For every render: create options, load fixed font bytes, parse SVG bytes, read the image size, allocate `width*height*4`, call `resvg_render`, convert premultiplied RGBA to Go NRGBA, and encode PNG. Match every C allocation/tree/options pointer with cleanup. Map every non-zero `resvg_error` to a named Go error.

- [ ] **Step 5: Build native code and confirm GREEN**

Run: `powershell -ExecutionPolicy Bypass -File native/resvg/build.ps1; go test ./... -run TestResvgRasterizerReturnsPNGAtSVGSize`

Expected: PASS.

- [ ] **Step 6: Write a failing browser-free renderer test**

```go
func TestRendererProducesPNGWithoutBrowser(t *testing.T) {
	r, err := NewRenderer()
	if err != nil { t.Fatal(err) }
	b, err := r.Render(context.Background(), []Message{{UserNickname: "张三", Avatar: fixtureDataURI(t, 42, 42), Message: "你好"}})
	if err != nil { t.Fatal(err) }
	if _, err := png.Decode(bytes.NewReader(b)); err != nil { t.Fatal(err) }
}
```

- [ ] **Step 7: Run it and confirm RED on the old constructor**

Run: `go test ./... -run TestRendererProducesPNGWithoutBrowser`

Expected: FAIL because `NewRenderer` still needs a browser pool.

- [ ] **Step 8: Replace orchestration**

```go
type Renderer struct {
	loader *ResourceLoader
	fonts *FontManager
	layout *LayoutEngine
	svg SVGBuilder
	rasterizer *ResvgRasterizer
	now func() time.Time
}
```

`NewRenderer()` initializes the fixed font, loader, layout engine, and rasterizer. `Render` keeps the eight-second timeout, logs individual remote-resource failures, lays out with `themeForTime(r.now())`, builds SVG, and rasterizes. `main.go` calls `NewRenderer()` directly. Delete browser initialization and pool files.

- [ ] **Step 9: Remove rod, verify, and commit**

Run: `go mod tidy; go test ./...; go list -m all | Select-String 'go-rod'`

Expected: tests PASS and dependency scan prints nothing.

```bash
git add resvg.go resvg.h resvg_test.go native/resvg renderer.go main.go go.mod go.sum
git rm pool.go pool_test.go
git commit -m "feat: replace chromium renderer with resvg"
```

### Task 6: Preserve Chromium and Add Visual Comparison

**Files:**
- Create: `tools/visual-regression/legacy/renderer.go`
- Create: `tools/visual-regression/legacy/pool.go`
- Create: `tools/visual-regression/legacy/template.go`
- Create: `tools/visual-regression/main.go`
- Create: `tools/visual-regression/compare.go`
- Create: `tools/visual-regression/compare_test.go`
- Create: `tools/visual-regression/go.mod`
- Create: `testdata/visual/README.md`

**Interfaces:**
- Produces: `Compare(image.Image, image.Image) Report` and old/new/diff PNG plus `report.json`.

- [ ] **Step 1: Copy the exact baseline into an isolated module**

Create one nested `tools/visual-regression` module. It depends on rod itself and uses a `replace` directive pointing to the repository root for the new renderer; therefore rod never enters the production module graph. Copy `renderer.go`, `pool.go`, and `template.go` from commit `49853a6` into its `legacy` package, changing only package boundaries and adding explicit theme injection. Preserve template CSS and screenshot behavior.

- [ ] **Step 2: Write failing comparison tests**

```go
func TestCompareReportsSizeAndPixelDifferences(t *testing.T) {
	a := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	b := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	b.SetNRGBA(1, 0, color.NRGBA{R: 255, A: 255})
	r := Compare(a, b)
	if !r.SameSize || r.ChangedPixels != 1 || r.TotalPixels != 2 { t.Fatalf("report = %#v", r) }
}
```

- [ ] **Step 3: Run it and confirm RED**

Run: `go -C tools/visual-regression test ./... -run TestCompare`

Expected: FAIL because `Compare` is absent.

- [ ] **Step 4: Implement raw reporting without heuristic thresholds**

```go
type Report struct {
	SameSize bool `json:"same_size"`
	WidthOld, HeightOld, WidthNew, HeightNew int
	ChangedPixels, TotalPixels int
	DifferenceRatio float64 `json:"difference_ratio"`
}
```

Decode to NRGBA, count exact changes, generate a red heatmap, and exit non-zero on dimension mismatch. Report facts; do not invent a passing threshold.

- [ ] **Step 5: Implement and run the dual-render command**

Accept `-fixture`, `-theme light|dark`, and `-out`; resolve fixture paths to data URIs once; write `chromium.png`, `resvg.png`, `diff.png`, and `report.json`.

Run:

```powershell
go -C tools/visual-regression run . -fixture ../../testdata/visual/messages.json -theme light -out ../../testdata/visual/out/light
go -C tools/visual-regression run . -fixture ../../testdata/visual/messages.json -theme dark -out ../../testdata/visual/out/dark
```

Expected: both exit 0, dimensions match, and all artifacts exist. For each proven geometry mismatch, first add a failing unit test, then correct only the responsible layout/SVG code.

- [ ] **Step 6: Commit**

```bash
git add tools/visual-regression testdata/visual/README.md
git commit -m "test: add chromium visual regression comparator"
```

### Task 7: Update Docker and README

**Files:**
- Modify: `Dockerfile`
- Modify: `README.md`
- Create: `config_test.go`

**Interfaces:**
- Produces: Chromium-free runtime image and accurate deployment docs.

- [ ] **Step 1: Add a failing configuration test**

```go
func TestProductionFilesDoNotReferenceChromiumPool(t *testing.T) {
	for _, path := range []string{"Dockerfile", "README.md", "main.go", "renderer.go"} {
		data, err := os.ReadFile(path)
		if err != nil { t.Fatal(err) }
		for _, forbidden := range []string{"POOL_SIZE", "ROD_BROWSER_BIN", "chromium"} {
			if bytes.Contains(bytes.ToLower(data), bytes.ToLower([]byte(forbidden))) { t.Fatalf("%s contains %q", path, forbidden) }
		}
	}
}
```

- [ ] **Step 2: Run it and confirm RED**

Run: `go test ./... -run TestProductionFilesDoNotReferenceChromiumPool`

Expected: FAIL on current deployment references.

- [ ] **Step 3: Replace build stages and documentation**

Docker uses a Rust native build stage, a CGO-enabled Go build stage, and a minimal runtime containing the linked library requirements and application only. Remove Chromium packages, rod variables, browser-pool documentation, and `POOL_SIZE`. Keep `PORT=5000`, `EXPOSE 5000`, and both endpoint examples.

- [ ] **Step 4: Build and manually smoke-test the container**

```powershell
docker build -t qq-quote-go:resvg .
docker run --rm -d --name qq-quote-resvg -p 18080:5000 qq-quote-go:resvg
curl.exe -f -X POST http://localhost:18080/png/ -H "Content-Type: application/json" --data-binary "@testdata/visual/messages.json" -o testdata/visual/docker.png
docker stop qq-quote-resvg
```

Expected: build succeeds, request returns 200, and output decodes as PNG. This is a manual smoke check, not an interface-test suite.

- [ ] **Step 5: Verify and commit**

Run: `gofmt -w *.go; go test ./...; go vet ./...; git diff --check`

Expected: all commands exit 0.

```bash
git add Dockerfile README.md config_test.go
git commit -m "docs: update deployment for resvg renderer"
```

### Task 8: Final Visual and Requirement Audit

**Files:**
- Modify only files with a reproduced mismatch and its failing regression test.
- Modify: `testdata/visual/README.md`

- [ ] **Step 1: Run fresh repository verification**

Run: `go test ./...; go vet ./...; git diff --check`

Expected: every command exits 0.

- [ ] **Step 2: Recreate light and dark visual outputs**

After verifying both resolved deletion targets remain below `testdata/visual/out`, remove only the previous light/dark output directories and rerun both Task 6 commands.

Expected: dimensions match; inspection finds no non-text geometry mismatch; remaining differences are confined to text-edge antialiasing.

- [ ] **Step 3: Audit the design conditions one by one**

Verify: production has no Chromium startup/dependency; all segment kinds render; failed resources continue; both endpoints retain methods/content types; `POOL_SIZE` is absent from production/deployment; tests pass; legacy rendering stays isolated.

- [ ] **Step 4: Record evidence and commit if files changed**

Add exact comparison commands and observed metrics to `testdata/visual/README.md`. Do not add heuristic pass thresholds or suppress mismatches.

```bash
git add testdata/visual/README.md
git commit -m "test: record resvg visual regression results"
```
