package main

import (
	"math"
	"testing"
)

func testLayoutEngine(t *testing.T) *LayoutEngine {
	t.Helper()
	fonts, err := NewFontManager(embeddedFont)
	if err != nil {
		t.Fatal(err)
	}
	return NewLayoutEngine(fonts)
}

func TestContainSizePreservesAspectRatio(t *testing.T) {
	w, h := containSize(400, 200, 200, 200)
	if w != 200 || h != 100 {
		t.Fatalf("got %v x %v", w, h)
	}
	w, h = containSize(100, 300, 200, 200)
	if math.Abs(w-66.6667) > .001 || h != 200 {
		t.Fatalf("got %v x %v", w, h)
	}
}

func TestLayoutUsesCurrentTemplateGeometry(t *testing.T) {
	card := testLayoutEngine(t).Layout([]PreparedMessage{{Nickname: "张三", Segments: []PreparedSegment{{Type: "text", Text: "你好"}}}}, darkTheme)
	row := card.Rows[0]
	if row.Avatar != (Rect{X: 12, Y: 16, W: 42, H: 42}) {
		t.Fatalf("avatar = %#v", row.Avatar)
	}
	if row.ContentX != 64 || row.Nickname.FontSize != 12 || row.Bubble.PaddingX != 12 || row.Bubble.PaddingY != 8 {
		t.Fatalf("row = %#v", row)
	}
}

func TestWrapTextPreservesNewlinesAndBreaksLongTokens(t *testing.T) {
	lines := testLayoutEngine(t).wrapText("第一行\nverylongunbrokenword", 60, 15)
	if len(lines) < 3 || lines[0].Text != "第一行" {
		t.Fatalf("lines = %#v", lines)
	}
}

func TestEmojiAndStickerUseCompactBounds(t *testing.T) {
	message := PreparedMessage{Segments: []PreparedSegment{
		{Type: "image", Kind: "emoji", Image: LoadedImage{Width: 80, Height: 80}},
		{Type: "image", Kind: "sticker", Image: LoadedImage{Width: 160, Height: 120}},
	}}
	card := testLayoutEngine(t).Layout([]PreparedMessage{message}, darkTheme)
	if got := card.Rows[0].Segments[0].Rect; got.W != 42 || got.H != 42 {
		t.Fatalf("emoji = %#v", got)
	}
	if got := card.Rows[0].Segments[1].Rect; got.W != 96 || got.H != 72 {
		t.Fatalf("sticker = %#v", got)
	}
}

func TestEmptyMessagesProducePaddedCard(t *testing.T) {
	card := testLayoutEngine(t).Layout(nil, darkTheme)
	if card.Width != 24 || card.Height != 32 {
		t.Fatalf("card = %#v", card)
	}
}

func TestRowsHaveFourteenPixelSpacing(t *testing.T) {
	messages := []PreparedMessage{{Segments: []PreparedSegment{{Type: "text", Text: "一"}}}, {Segments: []PreparedSegment{{Type: "text", Text: "二"}}}}
	card := testLayoutEngine(t).Layout(messages, lightTheme)
	a, b := card.Rows[0].Bounds, card.Rows[1].Bounds
	if b.Y-(a.Y+a.H) != 14 {
		t.Fatalf("rows = %#v", card.Rows)
	}
}
