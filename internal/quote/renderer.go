package quote

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/Penryn/qq-quote-generator/internal/resvg"
)

const defaultRenderTimeout = 8 * time.Second

type Renderer struct {
	loader     *ResourceLoader
	layout     *LayoutEngine
	svg        SVGBuilder
	rasterizer *resvg.Rasterizer
	now        func() time.Time
}

func NewRenderer() (*Renderer, error) {
	fonts, err := NewSystemFontManager(defaultFontFamilies)
	if err != nil {
		return nil, fmt.Errorf("font manager: %w", err)
	}
	rasterizer, err := resvg.NewRasterizer()
	if err != nil {
		return nil, fmt.Errorf("resvg: %w", err)
	}
	return &Renderer{
		loader: NewResourceLoader(&http.Client{Timeout: 5 * time.Second}, 16<<20),
		layout: NewLayoutEngine(fonts), svg: SVGBuilder{}, rasterizer: rasterizer, now: time.Now,
	}, nil
}

func (r *Renderer) Close() { r.rasterizer.Close() }

func (r *Renderer) Render(ctx context.Context, messages []Message) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultRenderTimeout)
	defer cancel()
	prepared, err := r.prepareMessages(ctx, messages)
	if err != nil {
		return nil, err
	}
	card := r.layout.Layout(prepared, themeForTime(r.now()))
	svg, err := r.svg.Build(card)
	if err != nil {
		return nil, fmt.Errorf("build SVG: %w", err)
	}
	png, err := r.rasterizer.Render(svg)
	if err != nil {
		return nil, fmt.Errorf("render SVG: %w", err)
	}
	return png, nil
}

func (r *Renderer) RenderBase64(ctx context.Context, messages []Message) (string, error) {
	png, err := r.Render(ctx, messages)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(png), nil
}

func (r *Renderer) prepareMessages(ctx context.Context, messages []Message) ([]PreparedMessage, error) {
	result := make([]PreparedMessage, 0, len(messages))
	for _, message := range messages {
		prepared := PreparedMessage{Nickname: message.UserNickname}
		prepared.Avatar = r.loadImage(ctx, safeImageURL(resolveAvatar(message)))
		segments, err := parseMessageField(message.Message)
		if err != nil {
			return nil, fmt.Errorf("parse message: %w", err)
		}
		for _, segment := range segments {
			item := PreparedSegment{Type: segment.Type, Kind: segment.Kind, Text: segment.Text}
			if segment.Type == "image" {
				item.Image = r.loadImage(ctx, safeImageURL(segment.URL))
			}
			prepared.Segments = append(prepared.Segments, item)
		}
		result = append(result, prepared)
	}
	return result, nil
}

func (r *Renderer) loadImage(ctx context.Context, source string) LoadedImage {
	image := r.loader.Load(ctx, source)
	if image.Err != nil {
		log.Printf("image unavailable (%s): %v", source, image.Err)
	}
	return image
}
