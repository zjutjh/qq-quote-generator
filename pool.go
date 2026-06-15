package main

import (
	"fmt"
	"log"

	"os"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

// BrowserPool 维护一组可复用的 rod.Page。
// 每个请求从池中取一个 Page，用完归还，实现真正并发。
type BrowserPool struct {
	browser *rod.Browser
	pool    chan *rod.Page
	size    int
}

// NewBrowserPool 启动 Chromium 并预热 size 个 Page。
func NewBrowserPool(size int) (*BrowserPool, error) {
	l := launcher.New().
		Headless(true).
		Set("disable-gpu", "").
		Set("no-sandbox", "").
		Set("disable-dev-shm-usage", "").
		Set("disable-extensions", "").
		Set("disable-background-networking", "").
		Set("disable-sync", "")

	if bin := os.Getenv("ROD_BROWSER_BIN"); bin != "" {
		l = l.Bin(bin)
	}

	u, err := l.Launch()
	if err != nil {
		return nil, fmt.Errorf("launcher: %w", err)
	}

	browser := rod.New().ControlURL(u).MustConnect()

	pool := make(chan *rod.Page, size)
	for i := 0; i < size; i++ {
		page, err := browser.Page(proto.TargetCreateTarget{URL: "about:blank"})
		if err != nil {
			return nil, fmt.Errorf("create page %d: %w", i, err)
		}
		// 固定视口宽度；高度由截图元素决定
		if err := page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{Width: 800, Height: 600, DeviceScaleFactor: 1}); err != nil {
			log.Printf("warn: set viewport: %v", err)
		}
		pool <- page
	}

	return &BrowserPool{
		browser: browser,
		pool:    pool,
		size:    size,
	}, nil
}

// Acquire 阻塞直到有空闲 Page 可用。
func (p *BrowserPool) Acquire() *rod.Page {
	return <-p.pool
}

// Release 将 Page 归还池中。
func (p *BrowserPool) Release(page *rod.Page) {
	p.pool <- page
}

// Close 关闭所有 Page 和浏览器进程。
func (p *BrowserPool) Close() {
	for i := 0; i < p.size; i++ {
		page := <-p.pool
		_ = page.Close()
	}
	_ = p.browser.Close()
}
