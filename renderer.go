package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"time"

	"github.com/go-rod/rod/lib/proto"
)

const defaultRenderTimeout = 8 * time.Second

// Renderer 持有 HTML 模板和 Page 池的引用
type Renderer struct {
	tmpl *template.Template
	pool *BrowserPool
}

func NewRenderer(pool *BrowserPool) (*Renderer, error) {
	tmpl, err := template.New("quote").Parse(quoteHTML)
	if err != nil {
		return nil, err
	}
	return &Renderer{tmpl: tmpl, pool: pool}, nil
}

// Render 处理一批消息，返回 PNG bytes
func (r *Renderer) Render(ctx context.Context, messages []Message) (png []byte, err error) {
	ctx, cancel := context.WithTimeout(ctx, defaultRenderTimeout)
	defer cancel()

	// 1. 预处理消息（解析 message 字段、拼装头像 URL）
	processed, err := r.processMessages(messages)
	if err != nil {
		return nil, err
	}

	// 2. 渲染 HTML 模板到字符串
	var buf bytes.Buffer
	if err := r.tmpl.Execute(&buf, renderData{Messages: processed, Theme: themeForTime(time.Now())}); err != nil {
		return nil, fmt.Errorf("template: %w", err)
	}
	html := buf.String()

	// 3. 从池中取一个 Page，注入 HTML，截图，归还
	page, err := r.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquire page: %w", err)
	}
	pageOK := false
	defer func() {
		if pageOK {
			r.pool.Release(page)
			return
		}
		r.pool.Replace(page)
	}()

	renderPage := page.Context(ctx)

	// SetContent 直接注入 HTML，完全避免本地 HTTP round trip
	// rod 的 Navigate + SetDocumentContent 方式
	if err := renderPage.Navigate("about:blank"); err != nil {
		return nil, fmt.Errorf("navigate: %w", err)
	}

	// 用 CDP 直接设置页面内容
	if err := renderPage.SetDocumentContent(html); err != nil {
		return nil, fmt.Errorf("setContent: %w", err)
	}

	// 等待页面短暂空闲；外部图片慢或失效时不能阻塞整个请求。
	_ = renderPage.WaitIdle(500 * time.Millisecond)

	// 只截取 #app 元素，高度自适应内容
	el, err := renderPage.Element("#app")
	if err != nil {
		return nil, fmt.Errorf("element #app: %w", err)
	}
	png, err = el.Screenshot(proto.PageCaptureScreenshotFormatPng, 90)
	if err != nil {
		return nil, fmt.Errorf("screenshot: %w", err)
	}

	pageOK = true
	return png, nil
}

func themeForTime(t time.Time) string {
	return themeForHour(t.Hour())
}

func themeForHour(hour int) string {
	if hour >= 6 && hour < 18 {
		return "theme-light"
	}
	return "theme-dark"
}

// RenderBase64 返回 base64 编码的 PNG
func (r *Renderer) RenderBase64(ctx context.Context, messages []Message) (string, error) {
	png, err := r.Render(ctx, messages)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(png), nil
}

// processMessages 将原始 Message 列表转换为模板可用的结构
func (r *Renderer) processMessages(messages []Message) ([]processedMessage, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	result := make([]processedMessage, 0, len(messages))

	for _, msg := range messages {
		pm := processedMessage{
			Nickname: msg.UserNickname,
			Avatar:   safeImageURL(resolveAvatar(client, msg)),
		}

		segs, err := parseMessageField(msg.Message)
		if err != nil {
			return nil, err
		}
		pm.Segments = processMessageSegments(segs)

		result = append(result, pm)
	}
	return result, nil
}

func processMessageSegments(segments []MessageSegment) []processedMessageSegment {
	result := make([]processedMessageSegment, 0, len(segments))
	for _, segment := range segments {
		result = append(result, processedMessageSegment{
			Type:       segment.Type,
			Kind:       segment.Kind,
			Text:       segment.Text,
			URL:        safeImageURL(segment.URL),
			ImageClass: imageClassForKind(segment.Kind),
		})
	}
	return result
}

func imageClassForKind(kind string) string {
	switch kind {
	case "emoji":
		return "bubble-img bubble-img-emoji"
	case "sticker":
		return "bubble-img bubble-img-sticker"
	default:
		return "bubble-img"
	}
}

func safeImageURL(raw string) template.URL {
	raw = strings.TrimSpace(raw)
	lower := strings.ToLower(raw)
	switch {
	case strings.HasPrefix(lower, "http://"), strings.HasPrefix(lower, "https://"), strings.HasPrefix(lower, "data:image/"):
		return template.URL(raw)
	default:
		return ""
	}
}

// resolveAvatar 返回可嵌入 <img src> 的头像值
func resolveAvatar(_ *http.Client, msg Message) string {
	if msg.Avatar != "" {
		return msg.Avatar
	}
	if msg.UserID > 0 {
		// QQ 头像 CDN，与原项目保持一致
		return fmt.Sprintf("https://q1.qlogo.cn/g?b=qq&nk=%d&s=100", msg.UserID)
	}
	return ""
}

// parseMessageField 兼容字符串和 []MessageSegment 两种格式
func parseMessageField(raw interface{}) ([]MessageSegment, error) {
	if raw == nil {
		return nil, nil
	}
	switch v := raw.(type) {
	case string:
		return []MessageSegment{{Type: "text", Text: v}}, nil
	case []interface{}:
		data, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		var segs []MessageSegment
		if err := json.Unmarshal(data, &segs); err != nil {
			return nil, err
		}
		return segs, nil
	default:
		return []MessageSegment{{Type: "text", Text: fmt.Sprintf("%v", v)}}, nil
	}
}
