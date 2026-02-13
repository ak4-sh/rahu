# Incremental Parsing

This document captures the analysis, design thinking, and implementation plan for
moving away from "full reparse on every keystroke" toward something smarter.

Last updated: 2026-02-12

---

## Current Status

**Important:** The AST cache bug (Step 1 below) has been FIXED! ✅

The `Server.SetAnalysis` method was implemented in `server/document.go:48-60` to
persist analysis results back to the stored Document. This means:
- The AST is now cached on the Document struct
- Hover and goto-definition can access the cached AST
- All LSP features that need the AST can read from the cache instead of re-parsing

---

## The Current Pipeline

Every `textDocument/didChange` notification triggers this sequence:

```
editor keystroke
  -> DidChange handler (server/handlers.go:28)
    -> ApplyFullChange (server/document.go:73) — updates stored text
    -> s.Get(uri) — returns a shallow copy of the Document
    -> s.analyze(doc) (server/analysis.go:9)
      -> parser.New(doc.Text) — creates a fresh lexer over the ENTIRE text
      -> p.Parse() — full recursive descent parse, produces *Module
      -> analyser.BuildScopes(module) — walks entire AST, builds scope tree
      -> analyser.Resolve(module, global) — walks entire AST again, resolves names
      -> toDiagnostics(...) — converts errors to LSP format
      -> publishDiagnostics(...) — sends to editor
```

This happens synchronously, on every single change event. For a user typing at
60 WPM (~5 characters per second), this means 5 full reparse cycles per second,
each one allocating an entirely new lexer, parser, AST, scope tree, and symbol map.

---

## Why This Matters

For small files (a few hundred lines), the current approach is likely fast enough —
a hand-written Go parser on a modern machine can probably parse several thousand
lines in well under a millisecond. But the approach has two deeper problems beyond
raw speed:

1. **Wasted work.** If the user types `def`, that's three change events (`d`, `de`,
   `def`). The first two produce broken ASTs that are immediately thrown away. Only
   the third one matters. We're doing 3x the work for 1x the value.

2. **The AST cache is broken.** `s.Get()` in `server/document.go:35` returns a
   shallow copy of the Document struct. When `analyze()` writes `doc.AST`,
   `doc.Symbols`, and `doc.SemErrs` to this copy, those values are never persisted
   back to the server's `docs` map. The stored Document always has nil AST/Symbols.
   This means any feature that needs the cached AST (hover, go-to-definition,
   completion) cannot access it — they would need to re-parse on every request too.

---

## Three Approaches Considered

### Approach 1: Debounce + Fix the Cache (recommended, do this first)

Don't change the parser at all. Instead, stop calling it so often and make sure
its results are actually saved.

**Why this works:** The parse itself is fast. The problem is calling it too
frequently and then throwing away the results. Debouncing reduces the call
frequency to roughly once per typing pause (50-100ms after the last keystroke).
Fixing the cache means features like hover can read the last successful AST
instead of re-parsing.

### Approach 2: Incremental Lexing + Full Reparse

Cache the token stream. On edit, re-lex only the affected region by restarting
the lexer from a known-good checkpoint. Then re-parse using the patched token
stream.

**Why this is hard:** The lexer's `indentStack` (tracking Python's significant
whitespace) is accumulated state. You cannot restart lexing from an arbitrary
point without knowing the indentation stack at that point. You would need to
store lexer state snapshots at regular intervals (e.g., every line boundary).
This is a moderate refactor of the lexer.

### Approach 3: Tree-sitter Style Incremental Parsing

Use a persistent/immutable tree where edits produce new trees that share
unchanged subtrees with the old tree. Each node stores byte offsets. On edit,
invalidate only the smallest affected subtree, re-lex and re-parse that region,
and stitch it back in.

**Why this is impractical right now:** This requires a fundamental redesign of
the AST (immutable nodes, byte offsets, parent pointers or zippers, stable node
identity). Tree-sitter's core engine is ~15K lines of C. Building this from
scratch is not justified at the current stage of the project.

---

## Recommended Plan: Approach 1 in Detail

### Step 1 — Fix the AST cache bug ✅ DONE

**Problem:** `analyze()` receives a shallow copy of the Document from `s.Get()`.
It writes `doc.AST`, `doc.Symbols`, and `doc.SemErrs` to the copy, which is then
discarded. The authoritative Document in `s.docs` never gets these fields set.

**Status:** FIXED in commit 2026-02-12

**What was changed:** Added `Server.SetAnalysis` method in `server/document.go:48-60`:

```go
// server/document.go

func (s *Server) SetAnalysis(
    uri lsp.DocumentURI,
    ast *parser.Module,
    symbols map[*parser.Name]*analyser.Symbol,
    semErrs []analyser.SemanticError,
) {
    s.mu.Lock()
    defer s.mu.Unlock()

    doc, ok := s.docs[uri]
    if !ok {
        return
    }

    doc.AST = ast
    doc.Symbols = symbols
    doc.SemErrs = semErrs
}
```

The `analyze()` function in `server/analysis.go` now calls `SetAnalysis` to persist results.

**Why this matters beyond caching:** Every LSP feature that needs the AST (hover,
completion, go-to-definition, references) can now read from the cached AST instead
of re-parsing the document.

---

### Step 2 — Debounce `DidChange` analysis (NOT YET IMPLEMENTED)

**Problem:** Every keystroke triggers a synchronous `analyze()` call. During fast
typing, most of these produce incomplete/broken ASTs that are immediately
superseded by the next keystroke's analysis.

**Design:** Use a per-document `time.Timer` that resets on each change. When the
timer fires (no new changes for N milliseconds), run `analyze()` in a goroutine.

**What to change in the Server struct:**

```go
// server/server.go

type Server struct {
    mu           sync.RWMutex
    docs         map[lsp.DocumentURI]*Document
    capabilities lsp.ClientCapabilities
    conn         *jsonrpc.Conn

    // debounce state
    timersMu sync.Mutex
    timers   map[lsp.DocumentURI]*time.Timer
}
```

Initialize `timers` in `New()`.

**What to change in the handlers:**

```go
// server/handlers.go

func (s *Server) DidOpen(p *lsp.DidOpenTextDocumentParams) {
    s.Open(p.TextDocument)
    doc := s.Get(p.TextDocument.URI)
    if doc != nil {
        // Analyze immediately on open — no debounce.
        // The user just opened the file and expects to see diagnostics.
        s.analyze(doc)
    }
}

func (s *Server) DidChange(p *lsp.DidChangeTextDocumentParams) {
    s.ApplyFullChange(
        p.TextDocument.URI,
        p.ContentChanges,
        p.TextDocument.Version,
    )

    s.scheduleAnalysis(p.TextDocument.URI)
}

func (s *Server) DidClose(p *lsp.DidCloseTextDocumentParams) {
    s.cancelAnalysis(p.TextDocument.URI)
    s.Close(p.TextDocument.URI)
}
```

**The debounce logic:**

```go
// server/analysis.go (or a new file, server/debounce.go)

const analyzeDelay = 80 * time.Millisecond

func (s *Server) scheduleAnalysis(uri lsp.DocumentURI) {
    s.timersMu.Lock()
    defer s.timersMu.Unlock()

    // Cancel any pending analysis for this document
    if t, ok := s.timers[uri]; ok {
        t.Stop()
    }

    s.timers[uri] = time.AfterFunc(analyzeDelay, func() {
        doc := s.Get(uri)
        if doc != nil {
            s.analyze(doc)
        }
    })
}

func (s *Server) cancelAnalysis(uri lsp.DocumentURI) {
    s.timersMu.Lock()
    defer s.timersMu.Unlock()

    if t, ok := s.timers[uri]; ok {
        t.Stop()
        delete(s.timers, uri)
    }
}
```

**Choosing the delay value:** 80ms is a reasonable starting point. At 60 WPM, the
average inter-keystroke interval is ~200ms, but during bursts (typing a familiar
word) it can drop to 50-80ms. An 80ms delay means:

- During fast typing: analysis is deferred until the burst ends
- During slow/thoughtful typing: analysis runs between keystrokes, ~80ms after each one
- The user perceives diagnostics appearing "immediately" after they pause

If diagnostics feel laggy, reduce to 50ms. If the server is doing too much work
during fast typing, increase to 100-150ms.

**Thread safety:** `time.AfterFunc` runs the callback in a new goroutine. The
`analyze()` function calls `s.Get()` (which takes `RLock`) and `s.SetAnalysis()`
(which takes `Lock`), and `s.publishDiagnostics()` (which calls `s.conn.Notify()`).
The JSON-RPC `Conn` must be safe for concurrent `Notify()` calls — check that
`conn.write()` uses a mutex or channel. If it doesn't, add one.

---

### Step 3 — Benchmark the pipeline

Before considering Approach 2, measure whether full reparse is actually a
bottleneck. The existing `parser/benchmark_test.go` references deleted fixture
files — it needs to be fixed first.

**What to benchmark:**

```go
// parser/benchmark_test.go (or server/analysis_test.go)

func BenchmarkFullPipeline(b *testing.B) {
    // Use a realistic Python file — 200-500 lines
    input := generatePythonFile(500) // or embed a fixture

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        p := parser.New(input)
        module := p.Parse()
        global := analyser.BuildScopes(module)
        analyser.Resolve(module, global)
    }
}
```

**What results mean:**

| Full pipeline time | Verdict |
|--------------------|---------|
| < 1ms for 500 lines | No further optimization needed. Debounce alone is sufficient. |
| 1-10ms for 500 lines | Fine for now. Revisit when supporting files > 2000 lines. |
| > 10ms for 500 lines | Something is wrong — profile and optimize hot paths before considering incremental parsing. |

For reference: Go's own parser processes ~10K lines in ~5ms. Our parser is
simpler (no type system, fewer AST nodes), so it should be faster per line.

---

## What This Unlocks

Once Steps 1-3 are done, the server will:

- **Stop wasting work** during fast typing
- **Maintain a valid cached AST** on each Document at all times ✅ (Step 1 done!)
- **Have measured data** on parse performance to guide future decisions

This directly unblocks:

- **Phase 3 (Real Hover):** The hover handler can read `doc.AST` and `doc.Symbols`
  instead of re-parsing (Step 1 done, implementation pending)
- **Phase 4 (Completion, Go-to-Definition, References):** 
  - ✅ Go-to-definition: IMPLEMENTED (uses cached AST)
  - Completion: needs cached AST, implementation pending
  - References: needs cached AST, implementation pending
- **Phase 5 (Performance):** The benchmarks from Step 3 will tell us whether
  Approach 2 is worth pursuing

---

## When to Revisit Incremental Parsing

Move to Approach 2 (incremental lexing) if **all three** of these are true:

1. Benchmarks show full reparse takes > 5ms for files users actually work with
2. Users report perceived lag in diagnostic updates
3. The debounce delay cannot be reduced further without causing excessive CPU usage

Until then, full reparse with debouncing is the right trade-off: simple code,
easy to debug, no architectural complexity, and fast enough for the common case.

---

## Appendix: Why Not Tree-sitter

Tree-sitter is sometimes held up as the gold standard for incremental parsing.
It achieves O(log n) re-parse cost by using a persistent tree structure where
edits produce new trees that share most of their nodes with the old tree.

This is impressive engineering, but it comes with significant costs:

- **Complexity:** The tree-sitter core is ~15K lines of C. The grammar DSL adds
  another layer. Reimplementing this in Go for a single language is not practical.
- **FFI option:** We could use tree-sitter's C library via cgo and its Python
  grammar. But this adds a C dependency, complicates the build, and means we have
  two parsers (tree-sitter for editing, ours for analysis) that must agree on
  semantics.
- **Our parser is already fast.** The value proposition of incremental parsing is
  "avoid reparsing unchanged regions." If reparsing the entire file takes < 1ms,
  the saved time is negligible compared to the added complexity.
- **Our AST carries semantic information.** Tree-sitter produces concrete syntax
  trees (every token is a node). Our parser produces abstract syntax trees with
  scope and symbol information baked in via the analyser. Bridging the two
  representations adds another translation layer.

Tree-sitter makes sense for editors that need to parse hundreds of languages with
real-time syntax highlighting. For a single-language LSP server with a fast
hand-written parser, it's overkill.
