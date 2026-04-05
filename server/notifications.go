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
	s.snapshotsMu.RLock()
	s.snapshotsMu.RUnlock()
	s.indexMu.RLock()
	count := len(s.modulesByName)
	s.indexMu.RUnlock()
	_ = s.conn.Notify("$/progress", lsp.ProgressParams{
		Token: indexingProgressToken(),
		Value: lsp.WorkDoneProgressEnd{
			Kind:    "end",
			Message: fmt.Sprintf("Finished indexing %d Python files", count),
		},
	})
}

func (s *Server) reportIndexingProgress(current, total int) {
	if s.conn == nil || total == 0 {
		return
	}
	percentage := uint32((current * 100) / total)
	_ = s.conn.Notify("$/progress", lsp.ProgressParams{
		Token: indexingProgressToken(),
		Value: lsp.WorkDoneProgressReport{
			Kind:       "report",
			Message:    fmt.Sprintf("Indexed %d/%d files", current, total),
			Percentage: &percentage,
		},
	})
}
