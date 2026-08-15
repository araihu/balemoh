//go:build ignore

package main

import (
	"bytes"
	"os"
)

func main() {
	path := "environment.md"
	contents, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}

	contents = append(bytes.TrimRight(contents, "\n"), '\n')
	if err := os.WriteFile(path, contents, 0o644); err != nil {
		panic(err)
	}
}
