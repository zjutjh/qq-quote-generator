package server

import (
	"context"
	"encoding/base64"
	"log"
	"net/http"

	"github.com/Penryn/qq-quote-generator/internal/quote"
	"github.com/gin-gonic/gin"
)

func New(renderer *quote.Renderer) *gin.Engine {
	router := gin.Default()
	_ = router.SetTrustedProxies(nil)

	render := func(contentType string, base64Output bool, renderImage func(context.Context, []quote.Message) ([]byte, error)) gin.HandlerFunc {
		return func(c *gin.Context) {
			var messages []quote.Message
			if err := c.ShouldBindJSON(&messages); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			data, err := renderImage(c.Request.Context(), messages)
			if err != nil {
				log.Printf("render error: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			if base64Output {
				c.String(http.StatusOK, base64.StdEncoding.EncodeToString(data))
			} else {
				c.Data(http.StatusOK, contentType, data)
			}
		}
	}
	router.POST("/png/", render("image/png", false, renderer.Render))
	router.POST("/png/base64/", render("image/png", true, renderer.Render))
	router.POST("/gif/", render("image/gif", false, renderer.RenderGIF))
	router.POST("/gif/base64/", render("image/gif", true, renderer.RenderGIF))

	return router
}
