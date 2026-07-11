package main

import (
	"bytes"
	"context"
	"image/png"
	"testing"
)

func TestResvgRasterizerReturnsPNGAtSVGSize(t *testing.T) {
	rasterizer, err := NewResvgRasterizer(embeddedFont)
	if err != nil {
		t.Fatal(err)
	}
	data, err := rasterizer.Render([]byte(`<svg xmlns="http://www.w3.org/2000/svg" width="7" height="5"><rect width="7" height="5" fill="#123456"/></svg>`))
	if err != nil {
		t.Fatal(err)
	}
	config, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if config.Width != 7 || config.Height != 5 {
		t.Fatalf("size = %dx%d", config.Width, config.Height)
	}
}

func TestRendererProducesPNGWithoutBrowser(t *testing.T) {
	renderer, err := NewRenderer()
	if err != nil {
		t.Fatal(err)
	}
	data, err := renderer.Render(context.Background(), []Message{{UserNickname: "张三", Avatar: fixtureDataURI(t, 42, 42), Message: "你好"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := png.Decode(bytes.NewReader(data)); err != nil {
		t.Fatal(err)
	}
}
