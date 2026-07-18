package quote

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/Penryn/qq-quote-generator/internal/resvg"
)

const (
	defaultRenderTimeout = 8 * time.Second
	outputScale          = 2.0
	qfaceBaseURL         = "https://koishi.js.org/QFace/assets/qq_emoji"
)

type Renderer struct {
	loader     *ResourceLoader
	layout     *LayoutEngine
	svg        SVGBuilder
	rasterizer *resvg.Rasterizer
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
		layout: NewLayoutEngine(fonts), svg: SVGBuilder{}, rasterizer: rasterizer,
	}, nil
}

func (r *Renderer) Close() { r.rasterizer.Close() }

func (r *Renderer) Render(ctx context.Context, messages []Message) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultRenderTimeout)
	defer cancel()
	prepared, err := r.prepareMessages(ctx, messages, false)
	if err != nil {
		return nil, err
	}
	card := r.layout.Layout(prepared)
	svg, err := r.svg.Build(card)
	if err != nil {
		return nil, fmt.Errorf("build SVG: %w", err)
	}
	png, err := r.rasterizer.Render(svg, outputScale)
	if err != nil {
		return nil, fmt.Errorf("render SVG: %w", err)
	}
	return png, nil
}

func (r *Renderer) RenderGIF(ctx context.Context, messages []Message) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultRenderTimeout)
	defer cancel()
	prepared, err := r.prepareMessages(ctx, messages, true)
	if err != nil {
		return nil, err
	}
	card := r.layout.Layout(prepared)
	return r.renderGIF(ctx, card)
}

func (r *Renderer) prepareMessages(ctx context.Context, messages []Message, animated bool) ([]PreparedMessage, error) {
	result := make([]PreparedMessage, 0, len(messages))
	for _, message := range messages {
		prepared := PreparedMessage{Nickname: message.UserNickname}
		prepared.Avatar = r.loadImage(ctx, safeImageURL(resolveAvatar(message)))
		prepared.Avatar.Data = nil
		segments, err := parseMessageField(message.Message)
		if err != nil {
			return nil, fmt.Errorf("parse message: %w", err)
		}
		for _, segment := range segments {
			item := PreparedSegment{Type: segment.Type, Kind: segment.Kind, Text: segment.Text}
			if segment.Type == "image" {
				item.Image = r.loadImage(ctx, safeImageURL(segment.URL))
			} else if segment.Type == "face" {
				id := faceID(segment.ID)
				if id == "" {
					log.Printf("invalid QQ face ID")
					item = PreparedSegment{Type: "text", Text: "[表情]"}
				} else {
					item = PreparedSegment{Type: "image", Kind: "emoji"}
					if animated {
						item.Image = r.loadImage(ctx, fmt.Sprintf("%s/%s/apng/%s.png", qfaceBaseURL, id, id))
					}
					if item.Image.DataURI == "" {
						item.Image = r.loadImage(ctx, fmt.Sprintf("%s/%s/png/%s.png", qfaceBaseURL, id, id))
					}
					if item.Image.DataURI == "" {
						item = PreparedSegment{Type: "text", Text: "[表情]"}
					}
				}
			}
			if !animated {
				item.Image.Data = nil
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
		label := source
		if strings.HasPrefix(strings.ToLower(label), "data:image/") {
			label = "data:image/*"
		}
		log.Printf("image unavailable (%s): %v", label, image.Err)
	}
	return image
}
