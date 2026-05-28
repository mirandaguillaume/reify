package main

import (
	"errors"
	"os"

	"github.com/mirandaguillaume/reify/internal/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		if errors.Is(err, cmd.ErrFindings) {
			os.Exit(2)
		}
		os.Exit(1)
	}
}
