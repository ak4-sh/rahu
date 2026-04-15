// Package rahu provides embedded access to builtin caches.
package rahu

import (
	"embed"
	"io/fs"
)

//go:embed all:builtin_cache
var BuiltinCacheFS embed.FS

// GetBuiltinCacheFS returns the embedded builtin cache filesystem.
func GetBuiltinCacheFS() fs.FS {
	return BuiltinCacheFS
}
