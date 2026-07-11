package server

import (
	"log"
	"net/http"

	"github.com/Penryn/qq-quote-generator/internal/quote"
	"github.com/gin-gonic/gin"
)

func New(renderer *quote.Renderer) *gin.Engine {
	router := gin.Default()
	_ = router.SetTrustedProxies(nil)

	router.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "see https://github.com/Penryn/qq-quote-generator")
	})

	router.POST("/png/", func(c *gin.Context) {
		var messages []quote.Message
		if err := c.ShouldBindJSON(&messages); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		data, err := renderer.Render(c.Request.Context(), messages)
		if err != nil {
			log.Printf("render error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.Data(http.StatusOK, "image/png", data)
	})

	router.POST("/base64/", func(c *gin.Context) {
		var messages []quote.Message
		if err := c.ShouldBindJSON(&messages); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		data, err := renderer.RenderBase64(c.Request.Context(), messages)
		if err != nil {
			log.Printf("render error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.String(http.StatusOK, data)
	})

	return router
}
