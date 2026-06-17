package main

import (
	"bytes"
	"html/template"
	"strings"
	"testing"
)

func TestTemplateUsesCircularAvatarAndContentWidth(t *testing.T) {
	if !strings.Contains(quoteHTML, "border-radius: 50%;") {
		t.Fatal("avatar should render as a circle")
	}
	if strings.Contains(quoteHTML, "min-width: 300px;") {
		t.Fatal("short quotes should not keep a fixed 300px minimum width")
	}
}

func TestThemeForHourUsesLightDuringDaytime(t *testing.T) {
	tests := map[int]string{
		5:  "theme-dark",
		6:  "theme-light",
		12: "theme-light",
		17: "theme-light",
		18: "theme-dark",
		23: "theme-dark",
	}

	for hour, want := range tests {
		if got := themeForHour(hour); got != want {
			t.Fatalf("themeForHour(%d) = %q, want %q", hour, got, want)
		}
	}
}

func TestTemplateIncludesLightAndDarkThemes(t *testing.T) {
	for _, want := range []string{".theme-light", ".theme-dark", `id="app" class="{{.Theme}}"`} {
		if !strings.Contains(quoteHTML, want) {
			t.Fatalf("template missing %q", want)
		}
	}
}

func TestTemplateAllowsDataURIImageSources(t *testing.T) {
	tmpl := template.Must(template.New("quote").Parse(quoteHTML))
	var buf bytes.Buffer
	err := tmpl.Execute(&buf, renderData{
		Theme: "theme-dark",
		Messages: []processedMessage{{
			Nickname: "张三",
			Avatar:   safeImageURL("data:image/png;base64,YXZhdGFy"),
			Segments: processMessageSegments([]MessageSegment{{
				Type: "image",
				URL:  "data:image/png;base64,cG5nLWRhdGE=",
			}}),
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	html := buf.String()
	if strings.Contains(html, "#ZgotmplZ") {
		t.Fatalf("template blocked a data URI: %s", html)
	}
	if !strings.Contains(html, "data:image/png;base64,cG5nLWRhdGE=") {
		t.Fatalf("template did not preserve image data URI: %s", html)
	}
}

func TestSafeImageURLRejectsScriptSources(t *testing.T) {
	if got := safeImageURL("javascript:alert(1)"); got != "" {
		t.Fatalf("safeImageURL accepted script source: %q", got)
	}
}
