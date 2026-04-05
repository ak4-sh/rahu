package server

import (
	"context"
	"fmt"
	"rahu/lsp"
	"time"
)

func indexingProgressToken() lsp.ProgressToken {
	return "rahu/indexing"
}

func (s *Server) createWorkspaceIndexingProgress() {
	if s.conn == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _ = s.conn.Request(ctx, "window/workDoneProgress/create", lsp.WorkDoneProgressCreateParams{
		Token: indexingProgressToken(),
	})
}

func (s *Server) beginWorkspaceIndexingProgress() {
	if s.conn == nil {
		return
	}
	_ = s.conn.Notify("$/progress", lsp.ProgressParams{
		Token: indexingProgressToken(),
		Value: lsp.WorkDoneProgressBegin{
			Kind:        "begin",
			Title:       "Indexing Python files",
			Cancellable: false,
		},
	})
}

func (s *Server) endWorkspaceIndexingProgress() {
	if s.conn == nil {
		return
	}
	s.mu.RLock()
	count := len(s.moduleSnapshotsByName)
	s.mu.RUnlock()
	_ = s.conn.Notify("$/progress", lsp.ProgressParams{
		Token: indexingProgressToken(),
		Value: lsp.WorkDoneProgressEnd{
			Kind:    "end",
			Message: fmt.Sprintf("Finished indexing %d Python files", count),
		},
	})
}
