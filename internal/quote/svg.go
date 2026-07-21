package quote

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
)

func buildSVG(card CardLayout) []byte {
	var out bytes.Buffer
	fmt.Fprintf(&out, `<svg xmlns="http://www.w3.org/2000/svg" width="%s" height="%s" viewBox="0 0 %s %s">`, px(card.Width), px(card.Height), px(card.Width), px(card.Height))
	fmt.Fprintf(&out, `<rect width="%s" height="%s" rx="12" fill="%s"/>`, px(card.Width), px(card.Height), cardBackground)
	out.WriteString(`<defs>`)
	for index, row := range card.Rows {
		radius := row.Avatar.W / 2
		fmt.Fprintf(&out, `<clipPath id="avatar-%d" clipPathUnits="userSpaceOnUse"><circle cx="%s" cy="%s" r="%s"/></clipPath>`, index, px(row.Avatar.X+radius), px(row.Avatar.Y+radius), px(radius))
	}
	out.WriteString(`</defs>`)
	for index, row := range card.Rows {
		writeRow(&out, index, row)
	}
	out.WriteString(`</svg>`)
	return out.Bytes()
}

func writeRow(out *bytes.Buffer, index int, row RowLayout) {
	radius := row.Avatar.W / 2
	fmt.Fprintf(out, `<circle cx="%s" cy="%s" r="%s" fill="%s"/>`, px(row.Avatar.X+radius), px(row.Avatar.Y+radius), px(radius), avatarBackground)
	if strings.HasPrefix(row.AvatarDataURI, "data:image/") {
		fmt.Fprintf(out, `<image x="%s" y="%s" width="%s" height="%s" href="%s" preserveAspectRatio="xMidYMid slice" clip-path="url(#avatar-%d)"/>`, px(row.Avatar.X), px(row.Avatar.Y), px(row.Avatar.W), px(row.Avatar.H), row.AvatarDataURI, index)
	}
	writeText(out, row.Nickname.Rect.X, row.Nickname.Rect.Y+row.Nickname.Lines[0].Baseline, nicknameColor, row.Nickname.Lines[0])
	writeBubble(out, row.Bubble.Rect, bubbleBackground)
	for segmentIndex, segment := range row.Segments {
		if segment.Type == "text" {
			for lineIndex, line := range segment.Lines {
				writeText(out, segment.Rect.X, segment.Rect.Y+line.Baseline+float64(lineIndex)*textLineHeight, messageColor, line)
			}
		} else if segment.Type == "image" && strings.HasPrefix(segment.DataURI, "data:image/") && segment.Rect.W > 0 && segment.Rect.H > 0 {
			clip := ""
			if imageHasRoundedCorners(segment.Kind) {
				id := fmt.Sprintf("message-image-%d-%d", index, segmentIndex)
				fmt.Fprintf(out, `<defs><clipPath id="%s" clipPathUnits="userSpaceOnUse"><rect x="%s" y="%s" width="%s" height="%s" rx="%s"/></clipPath></defs>`, id, px(segment.Rect.X), px(segment.Rect.Y), px(segment.Rect.W), px(segment.Rect.H), px(imageRadius))
				clip = ` clip-path="url(#` + id + `)"`
			}
			fmt.Fprintf(out, `<image x="%s" y="%s" width="%s" height="%s" href="%s" preserveAspectRatio="xMidYMid meet"%s/>`, px(segment.Rect.X), px(segment.Rect.Y), px(segment.Rect.W), px(segment.Rect.H), segment.DataURI, clip)
		}
	}
}

func writeBubble(out *bytes.Buffer, rect Rect, fill string) {
	x, y, right, bottom := rect.X, rect.Y, rect.X+rect.W, rect.Y+rect.H
	fmt.Fprintf(out, `<path d="M %s %s H %s Q %s %s %s %s V %s Q %s %s %s %s H %s Q %s %s %s %s V %s Q %s %s %s %s Z" fill="%s"/>`,
		px(x+4), px(y), px(right-12), px(right), px(y), px(right), px(y+12), px(bottom-12), px(right), px(bottom), px(right-12), px(bottom), px(x+12), px(x), px(bottom), px(x), px(bottom-12), px(y+4), px(x), px(y), px(x+4), px(y), fill)
}

func writeText(out *bytes.Buffer, x, y float64, color string, line TextLine) {
	for _, run := range line.Runs {
		fmt.Fprintf(out, `<text x="%s" y="%s" font-family="%s" font-size="%s" fill="%s">`, px(x), px(y), run.FontFamily, px(run.FontSize), color)
		_ = xml.EscapeText(out, []byte(run.Text))
		out.WriteString(`</text>`)
		x += run.Width
	}
}

func px(value float64) string { return strconv.FormatFloat(value, 'f', -1, 64) }
