package main

import (
	"log"

	"github.com/hzhyvinskyi/distrocommerce/internal/identity/app"
)

func main() {
	if err := app.Run(); err != nil {
		log.Fatalf("identity-sv: %v", err)
	}
}
