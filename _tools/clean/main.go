package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run _tools/clean <pattern1> [pattern2] ...")
		os.Exit(0)
	}

	for _, pattern := range os.Args[1:] {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			fmt.Printf("Error matching pattern %s: %v\n", pattern, err)
			continue
		}

		for _, match := range matches {
			err := os.RemoveAll(match)
			if err != nil {
				fmt.Printf("Error removing %s: %v\n", match, err)
			} else {
				fmt.Printf("Removed: %s\n", match)
			}
		}
	}
}
