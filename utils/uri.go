package utils

import (
	"net/url"
	"path/filepath"

	l "rahu/lsp"
)

func FilenameFromURI(uri l.DocumentURI) string {
	u, err := url.Parse(string(uri))
	if err != nil {
		return string(uri)
	}

	return filepath.Base(u.Path)
}
