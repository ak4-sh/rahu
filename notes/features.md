---

## Phase 0 – Stabilize the Core (COMPLETED)

**Goal:** reliable editor interaction

* Handshake works
* Hover stub works
* Document storage implemented
* `didChange` end-to-end works
* Version handling implemented
* Diagnostics publishing works

**Verification**

* Edit file in Neovim
* Diagnostics appear on save
* Edit triggers re-analysis

---

## Phase 1 – Text Synchronization (COMPLETED)

**Implement properly**

1. `textDocument/didChange`

   * call `ApplyFullChange` — implemented
   * incremental sync also implemented

2. `textDocument/didClose`

   * remove document state — implemented

3. Version checks — implemented

**Test cases**

* Open → edit → hover
* Multiple sequential edits
* Close → reopen

**Status: Done**

---

## Phase 2 – Diagnostics Foundation (COMPLETED)

**Goal:** show visible feedback in editor

1. Integrate lexer — done

   * tokenize document on change

2. Integrate parser — done

   * build AST

3. Implemented:

```
textDocument/publishDiagnostics
```

4. On every change:

```
text → parse → collect errors → publish
```

**Status: Done**

---

## Phase 3 – Real Hover (COMPLETED!)

Implementation:

1. Map LSP position → parser position ✅
2. Find Name node in AST via nameAtPos ✅
3. Look up symbol in doc.Symbols ✅
4. Return hover with symbol kind and definition location ✅

**Status: DONE** — hover now shows symbol kind and line number

---

## Phase 4 – Core Language Features (PARTIALLY COMPLETED)

Add in order:

1. ~~**Completion**~~ — not started

```
textDocument/completion
```

* based on AST symbols

2. ~~**Go to definition**~~ — DONE!

```
textDocument/definition
```

See [goto-definition-roadmap.md](./goto-definition-roadmap.md) for implementation details.

3. **References** — not started

```
textDocument/references
```

**Status: Go-to-definition implemented, others pending**

---

## Phase 5 – Performance

* Incremental parsing
* Background analysis queue
* Caching per document
* Debounce didChange events

---

## Phase 6 – Robustness

* Logging subsystem
* Panic recovery
* Error reporting
* Unit tests for:

  * edits
  * parser
  * handlers

---

## Phase 7 – Polish

* Incremental text sync
* Code actions
* Formatting
* Configuration options

---

## Immediate Next Action (today)

Implement real hover:

```
didChange → ApplyFullChange → hover shows symbol info
```

This builds on the AST position lookup already done for goto-definition.

