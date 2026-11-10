package main

import (
	"encoding/json"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/ai"
	"os"
)

func main() {
	kind := "journey"
	if len(os.Args) > 1 {
		kind = os.Args[1]
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(ai.Schema(kind)); err != nil {
		panic(err)
	}
}
