package main

import (
	"cmp"
	"log"
	"os"

	"github.com/Penryn/qq-quote-generator/internal/quote"
	"github.com/Penryn/qq-quote-generator/internal/server"
)

func main() {
	renderer, err := quote.NewRenderer()
	if err != nil {
		log.Fatalf("renderer: %v", err)
	}
	defer renderer.Close()

	port := cmp.Or(os.Getenv("PORT"), "5000")
	log.Printf("listening on :%s", port)
	if err := server.New(renderer).Run(":" + port); err != nil {
		log.Fatal(err)
	}
}
