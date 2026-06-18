package main

import (
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
)

func main() {
	poolSize := envInt("POOL_SIZE", 4)
	port := envStr("PORT", "5000")

	// 初始化 browser pool
	log.Printf("initializing browser pool (size=%d)...", poolSize)
	pool, err := NewBrowserPool(poolSize)
	if err != nil {
		log.Fatalf("browser pool: %v", err)
	}
	defer pool.Close()
	log.Println("browser pool ready")

	// 初始化 renderer
	renderer, err := NewRenderer(pool)
	if err != nil {
		log.Fatalf("renderer: %v", err)
	}

	// 路由
	r := gin.Default()
	r.SetTrustedProxies(nil)

	r.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "see https://github.com/zhullyb/qq-quote-generator")
	})

	// POST /png — 返回 PNG 图片
	r.POST("/png/", func(c *gin.Context) {
		var messages []Message
		if err := c.ShouldBindJSON(&messages); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		png, err := renderer.Render(c.Request.Context(), messages)
		if err != nil {
			log.Printf("render error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.Data(http.StatusOK, "image/png", png)
	})

	// POST /base64/ — 返回 base64 字符串
	r.POST("/base64/", func(c *gin.Context) {
		var messages []Message
		if err := c.ShouldBindJSON(&messages); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		b64, err := renderer.RenderBase64(c.Request.Context(), messages)
		if err != nil {
			log.Printf("render error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.String(http.StatusOK, b64)
	})

	log.Printf("listening on :%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func envStr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
