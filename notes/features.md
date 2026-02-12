---

## Phase 0 – Stabilize the Core (you are here)

**Goal:** reliable editor interaction

* Handshake already works
* Hover stub works
* Document storage implemented

**Remaining cleanup**

* Verify `didChange` end-to-end
* Ensure version handling
* Add basic logging

**Exit criteria**

* Edit file in Neovim
* Trigger hover
* Hover reflects latest text

---

# Phase 1 – Text Synchronization (highest priority)

**Implement properly**

1. `textDocument/didChange`

   * call `ApplyFullChange`
   * confirm updates work

2. `textDocument/didClose`

   * remove document state

3. Add version checks

**Test cases**

* Open → edit → hover
* Multiple sequential edits
* Close → reopen

**Exit criteria**

Server state always matches editor text.

---

# Phase 2 – Diagnostics Foundation

**Goal:** show visible feedback in editor

1. Integrate lexer

   * tokenize document on change

2. Integrate parser

   * build AST

3. Implement:

```
textDocument/publishDiagnostics
```

4. On every change:

```
text → parse → collect errors → publish
```

**Exit criteria**

Syntax errors appear in editor.

---

# Phase 3 – Real Hover

Replace stub logic:

1. Map LSP position → offset
2. Offset → token
3. Token → AST node
4. AST node → information string

Implement:

```
hover shows symbol under cursor
```

**Exit criteria**

Hover displays meaningful info.

---

# Phase 4 – Core Language Features

Add in order:

1. **Completion**

```
textDocument/completion
```

* based on AST symbols

2. **Go to definition**

```
textDocument/definition
```

See [goto-definition-roadmap.md](./goto-definition-roadmap.md) for implementation details.

3. **References**

```
textDocument/references
```

These three make the server actually productive.

---

# Phase 5 – Performance

* Incremental parsing
* Background analysis queue
* Caching per document
* Debounce didChange events

---

# Phase 6 – Robustness

* Logging subsystem
* Panic recovery
* Error reporting
* Unit tests for:

  * edits
  * parser
  * handlers

---

# Phase 7 – Polish

* Incremental text sync
* Code actions
* Formatting
* Configuration options

---

## Immediate Next Action (today)

Implement and verify:

```
didChange → ApplyFullChange → hover shows updated text
```

That unlocks all later features.

---

### Short roadmap summary

```
sync → diagnostics → real hover → completion → definition → references
```

Follow this order and progress will be steady and measurable.

