package main

import (
	"context"
	"testing"
	"time"

	"github.com/go-rod/rod"
)

func TestBrowserPoolAcquireHonorsContextTimeout(t *testing.T) {
	pool := &BrowserPool{pool: make(chan *rod.Page, 0)}

	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()

	_, err := pool.Acquire(ctx)
	if err == nil {
		t.Fatal("Acquire should return the context timeout error")
	}
}
