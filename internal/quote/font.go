package quote

import (
	"fmt"
	"sync"
	"unicode"

	"github.com/go-text/typesetting/di"
	"github.com/go-text/typesetting/font"
	"github.com/go-text/typesetting/fontscan"
	"github.com/go-text/typesetting/language"
	"github.com/go-text/typesetting/shaping"
	"golang.org/x/image/math/fixed"
)

var defaultFontFamilies = []string{"PingFang SC", "Microsoft YaHei", "Noto Sans CJK SC"}
var emojiFontFamilies = []string{"Apple Color Emoji", "Segoe UI Emoji"}

const emojiFontScale = 1.25

type FontManager struct {
	mu          sync.Mutex
	fonts       *fontscan.FontMap
	shaper      shaping.HarfbuzzShaper
	family      string
	emoji       fontscan.Location
	emojiFamily string
}

type fontMetrics struct {
	Width, Ascent, Descent float64
	Runs                   []textRun
}

type textRun struct {
	Text, FontFamily string
	Width, FontSize  float64
}

func NewSystemFontManager(families []string) (*FontManager, error) {
	fontMap := fontscan.NewFontMap(nil)
	if err := fontMap.UseSystemFonts(""); err != nil {
		return nil, fmt.Errorf("scan system fonts: %w", err)
	}
	selected := ""
	for _, family := range families {
		if _, ok := fontMap.FindSystemFont(family); ok {
			selected = family
			break
		}
	}
	if selected == "" {
		return nil, fmt.Errorf("none of the configured font families are installed: %v", families)
	}
	emojiFamily := ""
	var emojiLocation fontscan.Location
	for _, family := range emojiFontFamilies {
		if location, ok := fontMap.FindSystemFont(family); ok {
			emojiFamily, emojiLocation = family, location
			break
		}
	}
	if emojiFamily == "" {
		return nil, fmt.Errorf("none of the configured emoji font families are installed: %v", emojiFontFamilies)
	}
	aspect := font.Aspect{}
	aspect.SetDefaults()
	fontMap.SetQuery(fontscan.Query{Families: []string{selected, emojiFamily}, Aspect: aspect})
	return &FontManager{fonts: fontMap, family: selected, emoji: emojiLocation, emojiFamily: emojiFamily}, nil
}

func (m *FontManager) Measure(text string, size float64) float64 {
	return m.measureLine(text, size).Width
}

func (m *FontManager) measureLine(text string, size float64) fontMetrics {
	if text == "" {
		return fontMetrics{}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	runes := []rune(text)
	input := shaping.Input{Text: runes, RunStart: 0, RunEnd: len(runes), Direction: di.DirectionLTR, Size: fixed.Int26_6(size * 64), Language: language.DefaultLanguage()}
	var runs []shaping.Input
	for start := 0; start < len(runes); {
		end := start + 1
		space := unicode.IsSpace(runes[start])
		for end < len(runes) && unicode.IsSpace(runes[end]) == space {
			end++
		}
		part := input
		part.RunStart, part.RunEnd = start, end
		runs = append(runs, shaping.SplitByFace(part, m.fonts)...)
		start = end
	}
	var advance, ascent, descent fixed.Int26_6
	var textRuns []textRun
	for _, run := range runs {
		if run.RunStart < run.RunEnd {
			run.Script = language.LookupScript(runes[run.RunStart])
		}
		family := m.family
		fontSize := size
		if m.fonts.FontLocation(run.Face.Font) == m.emoji {
			family = m.emojiFamily
			fontSize *= emojiFontScale
			run.Size = fixed.Int26_6(fontSize * 64)
		}
		output := m.shaper.Shape(run)
		advance += output.Advance
		if output.GlyphBounds.Ascent > ascent {
			ascent = output.GlyphBounds.Ascent
		}
		if output.GlyphBounds.Descent < descent {
			descent = output.GlyphBounds.Descent
		}
		textRuns = append(textRuns, textRun{
			Text:       string(runes[run.RunStart:run.RunEnd]),
			FontFamily: family,
			Width:      float64(output.Advance) / 64,
			FontSize:   fontSize,
		})
	}
	return fontMetrics{
		Width:   float64(advance) / 64,
		Ascent:  float64(ascent) / 64,
		Descent: float64(descent) / 64,
		Runs:    textRuns,
	}
}
