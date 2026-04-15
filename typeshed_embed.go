// Package rahu embeds typeshed stubs into the binary.
// This file is at the root level to allow go:embed access to third_party directory.
package rahu

import (
	"embed"
	"io/fs"
	"log"
)

//go:embed all:third_party/typeshed
var typeshedFS embed.FS

// TypeshedFS returns the embedded typeshed filesystem (with root at third_party/typeshed stripped).
func TypeshedFS() fs.FS {
	// Strip the "third_party/typeshed" prefix to get direct access to stdlib/ and stubs/
	sub, err := fs.Sub(typeshedFS, "third_party/typeshed")
	if err != nil {
		log.Printf("[typeshed] Failed to create sub filesystem: %v", err)
		return typeshedFS // Return full FS as fallback
	}
	return sub
}
