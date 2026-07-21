package quote

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/color/palette"
	stdDraw "image/draw"
	"image/gif"
	"image/png"
	"log"
	"math"
	"slices"
	"strconv"

	"github.com/dgavrilov/imgpalette"
	"github.com/kettek/apng"
	"golang.org/x/image/draw"
)

const (
	maxSourceFrames       = 200
	maxDecodedPixelFrames = 16_000_000
	maxOutputPixelFrames  = 100_000_000
	maxGIFFrames          = 100
	maxGIFDuration        = 500 // centiseconds
	minFrameDelay         = 2
)

type decodedAnimation struct {
	frames []*image.NRGBA
	delays []int
	width  int
	height int
}

type gifPlacement struct {
	animation *decodedAnimation
	kind      string
	rect      image.Rectangle
	frames    []*image.NRGBA
}

type gifMoment struct {
	state []int
	delay int
}

func (r *Renderer) renderGIF(ctx context.Context, card CardLayout) ([]byte, error) {
	placements, err := collectAnimations(ctx, &card)
	if err != nil {
		return nil, err
	}
	svg := buildSVG(card)
	basePNG, err := r.rasterizer.Render(svg, outputScale)
	if err != nil {
		return nil, fmt.Errorf("render SVG: %w", err)
	}
	baseImage, err := png.Decode(bytes.NewReader(basePNG))
	if err != nil {
		return nil, fmt.Errorf("decode rendered PNG: %w", err)
	}
	base := toNRGBA(baseImage)
	preparePlacements(placements)

	maxFrames := maxGIFFrames
	if area := int64(base.Bounds().Dx()) * int64(base.Bounds().Dy()); area > 0 {
		maxFrames = min(maxFrames, max(1, int(maxOutputPixelFrames/area)))
	}
	moments := buildTimeline(placements, maxFrames)
	gifPalette := buildGIFPalette(base, placements)
	output := gif.GIF{
		Image:     make([]*image.Paletted, 0, len(moments)),
		Delay:     make([]int, 0, len(moments)),
		Disposal:  make([]byte, 0, len(moments)),
		LoopCount: 0,
		Config: image.Config{
			ColorModel: gifPalette,
			Width:      base.Bounds().Dx(),
			Height:     base.Bounds().Dy(),
		},
	}
	for index, moment := range moments {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		bounds := base.Bounds()
		if index > 0 {
			bounds = changedBounds(placements, moments[index-1].state, moment.state)
			if bounds.Empty() {
				output.Delay[len(output.Delay)-1] += moment.delay
				continue
			}
		}
		frame := composeFrame(base, placements, moment.state, bounds)
		paletted := image.NewPaletted(bounds, gifPalette)
		stdDraw.FloydSteinberg.Draw(paletted, bounds, frame, bounds.Min)
		output.Image = append(output.Image, paletted)
		output.Delay = append(output.Delay, moment.delay)
		output.Disposal = append(output.Disposal, gif.DisposalNone)
	}
	var encoded bytes.Buffer
	if err := gif.EncodeAll(&encoded, &output); err != nil {
		return nil, fmt.Errorf("encode GIF: %w", err)
	}
	return encoded.Bytes(), nil
}

func collectAnimations(ctx context.Context, card *CardLayout) ([]*gifPlacement, error) {
	var placements []*gifPlacement
	frames, pixels := 0, int64(0)
	for rowIndex := range card.Rows {
		for segmentIndex := range card.Rows[rowIndex].Segments {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			segment := &card.Rows[rowIndex].Segments[segmentIndex]
			if segment.Type != "image" || len(segment.ImageData) == 0 {
				continue
			}
			data := segment.ImageData
			segment.ImageData = nil
			animation, err := decodeAnimation(data)
			if err != nil {
				log.Printf("animation unavailable: %v", err)
				continue
			}
			if animation == nil {
				continue
			}
			animationPixels := int64(animation.width) * int64(animation.height) * int64(len(animation.frames))
			if frames+len(animation.frames) > maxSourceFrames || pixels+animationPixels > maxDecodedPixelFrames {
				log.Printf("animation exceeds decoded frame or pixel limit; using static image")
				continue
			}
			frames += len(animation.frames)
			pixels += animationPixels
			segment.DataURI = ""
			placements = append(placements, &gifPlacement{animation: animation, kind: segment.Kind, rect: logicalToPhysical(segment.Rect)})
		}
	}
	return placements, nil
}

func decodeAnimation(data []byte) (*decodedAnimation, error) {
	if len(data) >= 6 && (bytes.Equal(data[:6], []byte("GIF87a")) || bytes.Equal(data[:6], []byte("GIF89a"))) {
		return decodeGIF(data)
	}
	if len(data) >= 8 && bytes.Equal(data[:8], []byte("\x89PNG\r\n\x1a\n")) {
		return decodeAPNG(data)
	}
	return nil, nil
}

func decodeGIF(data []byte) (*decodedAnimation, error) {
	decoded, err := gif.DecodeAll(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode GIF: %w", err)
	}
	if len(decoded.Image) < 2 {
		return nil, nil
	}
	canvas := image.NewNRGBA(image.Rect(0, 0, decoded.Config.Width, decoded.Config.Height))
	animation := &decodedAnimation{width: decoded.Config.Width, height: decoded.Config.Height}
	background := image.Transparent
	if colors, ok := decoded.Config.ColorModel.(color.Palette); ok && int(decoded.BackgroundIndex) < len(colors) {
		background = image.NewUniform(colors[decoded.BackgroundIndex])
	}
	for index, source := range decoded.Image {
		disposal := decoded.Disposal[index]
		var previous *image.NRGBA
		if disposal == gif.DisposalPrevious {
			previous = cloneNRGBA(canvas)
		}
		stdDraw.Draw(canvas, source.Bounds(), source, source.Bounds().Min, stdDraw.Over)
		animation.frames = append(animation.frames, cloneNRGBA(canvas))
		animation.delays = append(animation.delays, normalizeDelay(decoded.Delay[index]))
		switch disposal {
		case gif.DisposalBackground:
			stdDraw.Draw(canvas, source.Bounds(), background, image.Point{}, stdDraw.Src)
		case gif.DisposalPrevious:
			canvas = previous
		}
	}
	return animation, nil
}

func decodeAPNG(data []byte) (*decodedAnimation, error) {
	config, err := apng.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode APNG config: %w", err)
	}
	decoded, err := apng.DecodeAll(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode APNG: %w", err)
	}
	canvas := image.NewNRGBA(image.Rect(0, 0, config.Width, config.Height))
	animation := &decodedAnimation{width: config.Width, height: config.Height}
	for index := range decoded.Frames {
		source := &decoded.Frames[index]
		if source.IsDefault {
			continue
		}
		var previous *image.NRGBA
		if source.DisposeOp == apng.DISPOSE_OP_PREVIOUS {
			previous = cloneNRGBA(canvas)
		}
		bounds := source.Image.Bounds()
		target := image.Rect(source.XOffset, source.YOffset, source.XOffset+bounds.Dx(), source.YOffset+bounds.Dy())
		op := stdDraw.Over
		if source.BlendOp == apng.BLEND_OP_SOURCE {
			op = stdDraw.Src
		}
		stdDraw.Draw(canvas, target, source.Image, bounds.Min, op)
		animation.frames = append(animation.frames, cloneNRGBA(canvas))
		animation.delays = append(animation.delays, normalizeDelay(int(math.Round(source.GetDelay()*100))))
		switch source.DisposeOp {
		case apng.DISPOSE_OP_BACKGROUND:
			stdDraw.Draw(canvas, target, image.Transparent, image.Point{}, stdDraw.Src)
		case apng.DISPOSE_OP_PREVIOUS:
			canvas = previous
		}
	}
	if len(animation.frames) < 2 {
		return nil, nil
	}
	return animation, nil
}

func preparePlacements(placements []*gifPlacement) {
	for _, placement := range placements {
		target := placement.rect
		if placement.kind == "emoji" {
			target = containRect(image.Rect(0, 0, placement.animation.width, placement.animation.height), target)
			placement.rect = target
		}
		if target.Empty() {
			continue
		}
		placement.frames = make([]*image.NRGBA, len(placement.animation.frames))
		for index, source := range placement.animation.frames {
			frame := image.NewNRGBA(image.Rect(0, 0, target.Dx(), target.Dy()))
			draw.CatmullRom.Scale(frame, frame.Bounds(), source, source.Bounds(), stdDraw.Over, nil)
			if imageHasRoundedCorners(placement.kind) {
				applyRoundedCorners(frame, int(math.Round(imageRadius*outputScale)))
			}
			placement.frames[index] = frame
		}
	}
}

func logicalToPhysical(rect Rect) image.Rectangle {
	return image.Rect(
		int(math.Round(rect.X*outputScale)),
		int(math.Round(rect.Y*outputScale)),
		int(math.Round((rect.X+rect.W)*outputScale)),
		int(math.Round((rect.Y+rect.H)*outputScale)),
	)
}

func buildTimeline(placements []*gifPlacement, maxFrames int) []gifMoment {
	if len(placements) == 0 {
		return []gifMoment{{delay: 0}}
	}
	period, tick := 1, 0
	for _, placement := range placements {
		cycle := 0
		for _, delay := range placement.animation.delays {
			cycle += delay
			tick = gcd(tick, delay)
		}
		period = lcmCapped(period, cycle, maxGIFDuration)
	}
	period = min(period, maxGIFDuration)
	if tick <= 0 {
		tick = minFrameDelay
	}
	if (period+tick-1)/tick > maxFrames {
		tick = (period + maxFrames - 1) / maxFrames
	}
	var moments []gifMoment
	for elapsed := 0; elapsed < period; elapsed += tick {
		state := make([]int, len(placements))
		for index, placement := range placements {
			state[index] = frameAt(placement.animation.delays, elapsed)
		}
		delay := min(tick, period-elapsed)
		if len(moments) > 0 && slices.Equal(moments[len(moments)-1].state, state) {
			moments[len(moments)-1].delay += delay
		} else {
			moments = append(moments, gifMoment{state: state, delay: delay})
		}
	}
	return moments
}

func frameAt(delays []int, elapsed int) int {
	cycle := 0
	for _, delay := range delays {
		cycle += delay
	}
	if cycle == 0 {
		return 0
	}
	elapsed %= cycle
	for index, delay := range delays {
		if elapsed < delay {
			return index
		}
		elapsed -= delay
	}
	return len(delays) - 1
}

func buildGIFPalette(base *image.NRGBA, placements []*gifPlacement) color.Palette {
	sample := image.NewNRGBA(image.Rect(0, 0, 1024, 1024))
	baseTarget := containRect(base.Bounds(), image.Rect(0, 0, 512, 1024))
	draw.CatmullRom.Scale(sample, baseTarget, base, base.Bounds(), stdDraw.Over, nil)
	var frames []*image.NRGBA
	for _, placement := range placements {
		frames = append(frames, placement.frames...)
	}
	if len(frames) > 128 {
		selected := make([]*image.NRGBA, 0, 128)
		for index := range 128 {
			selected = append(selected, frames[index*len(frames)/128])
		}
		frames = selected
	}
	for index, frame := range frames {
		x := 512 + (index%8)*64
		y := (index / 8) * 64
		draw.CatmullRom.Scale(sample, image.Rect(x, y, x+64, y+64), frame, frame.Bounds(), stdDraw.Over, nil)
	}
	result := color.Palette{color.NRGBA{A: 0}}
	for _, value := range []string{cardBackground, avatarBackground, nicknameColor, bubbleBackground, messageColor} {
		result = appendUniqueColor(result, hexColor(value))
	}
	extracted, err := imgpalette.Extract(sample, imgpalette.Count(256-len(result)), imgpalette.Resize(1024), imgpalette.Space(imgpalette.SpaceOKLab))
	if err == nil {
		for _, value := range extracted.Colors() {
			result = appendUniqueColor(result, value)
		}
	}
	for _, value := range palette.Plan9 {
		if len(result) == 256 {
			break
		}
		result = appendUniqueColor(result, value)
	}
	return result
}

func appendUniqueColor(palette color.Palette, value color.Color) color.Palette {
	if value == nil || len(palette) >= 256 {
		return palette
	}
	r, g, b, a := value.RGBA()
	for _, existing := range palette {
		er, eg, eb, ea := existing.RGBA()
		if r == er && g == eg && b == eb && a == ea {
			return palette
		}
	}
	return append(palette, value)
}

func hexColor(value string) color.NRGBA {
	parsed, _ := strconv.ParseUint(value[1:], 16, 24)
	return color.NRGBA{R: uint8(parsed >> 16), G: uint8(parsed >> 8), B: uint8(parsed), A: 0xff}
}

func composeFrame(base *image.NRGBA, placements []*gifPlacement, state []int, bounds image.Rectangle) *image.NRGBA {
	frame := image.NewNRGBA(bounds)
	stdDraw.Draw(frame, bounds, base, bounds.Min, stdDraw.Src)
	for index, placement := range placements {
		if len(placement.frames) == 0 || !placement.rect.Overlaps(bounds) {
			continue
		}
		source := placement.frames[state[index]]
		target := placement.rect.Intersect(bounds)
		point := image.Pt(target.Min.X-placement.rect.Min.X, target.Min.Y-placement.rect.Min.Y)
		stdDraw.Draw(frame, target, source, point, stdDraw.Over)
	}
	return frame
}

func changedBounds(placements []*gifPlacement, previous, current []int) image.Rectangle {
	var bounds image.Rectangle
	for index, placement := range placements {
		if previous[index] == current[index] {
			continue
		}
		if bounds.Empty() {
			bounds = placement.rect
		} else {
			bounds = bounds.Union(placement.rect)
		}
	}
	return bounds
}

func containRect(source, target image.Rectangle) image.Rectangle {
	scale := math.Min(float64(target.Dx())/float64(source.Dx()), float64(target.Dy())/float64(source.Dy()))
	width := int(math.Round(float64(source.Dx()) * scale))
	height := int(math.Round(float64(source.Dy()) * scale))
	x := target.Min.X + (target.Dx()-width)/2
	y := target.Min.Y + (target.Dy()-height)/2
	return image.Rect(x, y, x+width, y+height)
}

func applyRoundedCorners(frame *image.NRGBA, radius int) {
	radius = min(radius, min(frame.Bounds().Dx(), frame.Bounds().Dy())/2)
	if radius <= 0 {
		return
	}
	width, height := frame.Bounds().Dx(), frame.Bounds().Dy()
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			cx, cy := x, y
			if x >= radius && x < width-radius || y >= radius && y < height-radius {
				continue
			}
			if x >= width-radius {
				cx = width - 1 - x
			}
			if y >= height-radius {
				cy = height - 1 - y
			}
			dx, dy := float64(radius-cx)-0.5, float64(radius-cy)-0.5
			if dx*dx+dy*dy > float64(radius*radius) {
				frame.Pix[frame.PixOffset(x, y)+3] = 0
			}
		}
	}
}

func toNRGBA(source image.Image) *image.NRGBA {
	if source, ok := source.(*image.NRGBA); ok {
		return source
	}
	target := image.NewNRGBA(source.Bounds())
	stdDraw.Draw(target, target.Bounds(), source, source.Bounds().Min, stdDraw.Src)
	return target
}

func cloneNRGBA(source *image.NRGBA) *image.NRGBA {
	target := image.NewNRGBA(source.Bounds())
	stdDraw.Draw(target, target.Bounds(), source, source.Bounds().Min, stdDraw.Src)
	return target
}

func normalizeDelay(delay int) int { return min(max(delay, minFrameDelay), maxGIFDuration) }

func gcd(left, right int) int {
	for right != 0 {
		left, right = right, left%right
	}
	return left
}

func lcmCapped(left, right, cap int) int {
	if left == 0 || right == 0 {
		return 0
	}
	divisor := gcd(left, right)
	if left/divisor > cap/right {
		return cap
	}
	return min(left/divisor*right, cap)
}
