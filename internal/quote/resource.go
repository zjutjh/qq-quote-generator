package quote

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/jpeg"
	"io"
	"mime"
	"net/http"
	"strings"
)

type LoadedImage struct {
	DataURI string
	Data    []byte
	Width   int
	Height  int
	Err     error
}

type ResourceLoader struct {
	client   *http.Client
	maxBytes int64
}

func NewResourceLoader(client *http.Client, maxBytes int64) *ResourceLoader {
	return &ResourceLoader{client: client, maxBytes: maxBytes}
}

func (l *ResourceLoader) Load(ctx context.Context, source string) LoadedImage {
	if source == "" {
		return LoadedImage{}
	}
	data, mediaType, err := l.read(ctx, source)
	if err != nil {
		return LoadedImage{Err: err}
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return LoadedImage{Err: fmt.Errorf("decode image config: %w", err)}
	}
	return LoadedImage{
		DataURI: "data:" + mediaType + ";base64," + base64.StdEncoding.EncodeToString(data),
		Data:    data,
		Width:   config.Width,
		Height:  config.Height,
	}
}

func (l *ResourceLoader) read(ctx context.Context, source string) ([]byte, string, error) {
	if strings.HasPrefix(strings.ToLower(source), "data:image/") {
		return l.readDataURI(source)
	}
	return l.readHTTP(ctx, source)
}

func (l *ResourceLoader) readDataURI(source string) ([]byte, string, error) {
	header, payload, ok := strings.Cut(source, ",")
	if !ok {
		return nil, "", fmt.Errorf("invalid image data URI")
	}
	metadata := strings.TrimPrefix(header, "data:")
	if !strings.HasSuffix(strings.ToLower(metadata), ";base64") {
		return nil, "", fmt.Errorf("image data URI must be base64 encoded")
	}
	mediaType, _, err := mime.ParseMediaType(metadata[:len(metadata)-len(";base64")])
	if err != nil || !strings.HasPrefix(strings.ToLower(mediaType), "image/") {
		return nil, "", fmt.Errorf("invalid image data URI media type")
	}
	data, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return nil, "", fmt.Errorf("decode image data URI: %w", err)
	}
	if int64(len(data)) > l.maxBytes {
		return nil, "", fmt.Errorf("image exceeds %d byte limit", l.maxBytes)
	}
	return data, mediaType, nil
}

func (l *ResourceLoader) readHTTP(ctx context.Context, source string) ([]byte, string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
	if err != nil {
		return nil, "", fmt.Errorf("create image request: %w", err)
	}
	response, err := l.client.Do(request)
	if err != nil {
		return nil, "", fmt.Errorf("load image: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, "", fmt.Errorf("load image: HTTP %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, l.maxBytes+1))
	if err != nil {
		return nil, "", fmt.Errorf("read image: %w", err)
	}
	if int64(len(data)) > l.maxBytes {
		return nil, "", fmt.Errorf("image exceeds %d byte limit", l.maxBytes)
	}
	mediaType, _, _ := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if !strings.HasPrefix(strings.ToLower(mediaType), "image/") {
		mediaType = http.DetectContentType(data)
	}
	if !strings.HasPrefix(strings.ToLower(mediaType), "image/") {
		return nil, "", fmt.Errorf("resource is not an image")
	}
	return data, mediaType, nil
}
