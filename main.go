package main

import (
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

func main() {
	port := envStr("PORT", "5000")

	renderer, err := NewRenderer()
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

func envStr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
