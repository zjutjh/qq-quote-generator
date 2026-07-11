package quote

import (
	"fmt"
	"sync"

	"github.com/go-text/typesetting/di"
	"github.com/go-text/typesetting/font"
	"github.com/go-text/typesetting/fontscan"
	"github.com/go-text/typesetting/language"
	"github.com/go-text/typesetting/shaping"
	"golang.org/x/image/math/fixed"
)

var defaultFontFamilies = []string{"PingFang SC", "Microsoft YaHei", "Noto Sans CJK SC"}

type FontManager struct {
	mu     sync.Mutex
	fonts  *fontscan.FontMap
	shaper shaping.HarfbuzzShaper
	family string
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
	aspect := font.Aspect{}
	aspect.SetDefaults()
	fontMap.SetQuery(fontscan.Query{Families: []string{selected}, Aspect: aspect})
	return &FontManager{fonts: fontMap, family: selected}, nil
}

func (m *FontManager) Family() string { return m.family }

func (m *FontManager) Measure(text string, size float64) float64 {
	if text == "" {
		return 0
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	runes := []rune(text)
	input := shaping.Input{Text: runes, RunStart: 0, RunEnd: len(runes), Direction: di.DirectionLTR, Size: fixed.Int26_6(size * 64), Language: language.DefaultLanguage()}
	runs := shaping.SplitByFace(input, m.fonts)
	var advance fixed.Int26_6
	for _, run := range runs {
		if run.RunStart < run.RunEnd {
			run.Script = language.LookupScript(runes[run.RunStart])
		}
		advance += m.shaper.Shape(run).Advance
	}
	return float64(advance) / 64
}
