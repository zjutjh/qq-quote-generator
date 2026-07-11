package resvg

/*
#cgo windows,amd64 LDFLAGS: -L${SRCDIR}/lib/windows-amd64 -lresvg -lkernel32 -lntdll -luserenv -lws2_32 -ldbghelp
#cgo linux,amd64 LDFLAGS: -L${SRCDIR}/lib/linux-amd64 -lresvg -lgcc_s -lutil -lrt -lpthread -lm -ldl
#cgo darwin,amd64 LDFLAGS: -L${SRCDIR}/lib/darwin-amd64 -lresvg -framework CoreFoundation -framework Security -liconv
#cgo darwin,arm64 LDFLAGS: -L${SRCDIR}/lib/darwin-arm64 -lresvg -framework CoreFoundation -framework Security -liconv
#include "resvg.h"
*/
import "C"

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"math"
	"sync"
	"unsafe"
)

type Rasterizer struct {
	mu      sync.RWMutex
	options *C.resvg_options
}

func NewRasterizer() (*Rasterizer, error) {
	options := C.resvg_options_create()
	if options == nil {
		return nil, fmt.Errorf("create resvg options")
	}
	C.resvg_options_load_system_fonts(options)
	return &Rasterizer{options: options}, nil
}

func (r *Rasterizer) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.options != nil {
		C.resvg_options_destroy(r.options)
		r.options = nil
	}
}

func (r *Rasterizer) Render(svg []byte) ([]byte, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.options == nil {
		return nil, fmt.Errorf("resvg rasterizer is closed")
	}
	if len(svg) == 0 {
		return nil, fmt.Errorf("resvg input is empty")
	}

	var tree *C.resvg_render_tree
	result := C.resvg_parse_tree_from_data((*C.char)(unsafe.Pointer(&svg[0])), C.uintptr_t(len(svg)), r.options, &tree)
	if result != C.RESVG_OK {
		return nil, fmt.Errorf("parse SVG: resvg error %d", int32(result))
	}
	defer C.resvg_tree_destroy(tree)
	size := C.resvg_get_image_size(tree)
	width, height := int(math.Ceil(float64(size.width))), int(math.Ceil(float64(size.height)))
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("resvg returned invalid size %dx%d", width, height)
	}
	pixels := make([]byte, width*height*4)
	transform := C.resvg_transform_identity()
	C.resvg_render(tree, transform, C.uint32_t(width), C.uint32_t(height), (*C.char)(unsafe.Pointer(&pixels[0])))
	unpremultiply(pixels)
	imageData := &image.NRGBA{Pix: pixels, Stride: width * 4, Rect: image.Rect(0, 0, width, height)}
	var output bytes.Buffer
	if err := png.Encode(&output, imageData); err != nil {
		return nil, fmt.Errorf("encode PNG: %w", err)
	}
	return output.Bytes(), nil
}

func unpremultiply(pixels []byte) {
	for index := 0; index < len(pixels); index += 4 {
		alpha := uint32(pixels[index+3])
		if alpha == 0 || alpha == 255 {
			continue
		}
		pixels[index] = uint8(min(255, (uint32(pixels[index])*255+alpha/2)/alpha))
		pixels[index+1] = uint8(min(255, (uint32(pixels[index+1])*255+alpha/2)/alpha))
		pixels[index+2] = uint8(min(255, (uint32(pixels[index+2])*255+alpha/2)/alpha))
	}
}
