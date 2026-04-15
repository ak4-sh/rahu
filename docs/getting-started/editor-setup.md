# Editor Setup

Rahu implements the Language Server Protocol (LSP), so it works with any LSP-compatible editor.

## VS Code

### Using settings.json

Add to your VS Code `settings.json`:

```json
{
  "python.languageServer": "none",
  "python.analysis.typeCheckingMode": "off",
  "python.lsp.extraPaths": [],
  "python.lsp.server": {
    "command": ["rahu"],
    "args": [],
    "filetypes": ["python"]
  }
}
```

Or use the generic LSP client:

```json
{
  "languageServerExample.trace.server": "verbose",
  "python.lsp.server": {
    "command": ["/path/to/rahu/cmd/lsp"],
    "args": [],
    "filetypes": ["python"],
    "initializationOptions": {},
    "settings": {}
  }
}
```

### Using a VS Code Extension

If someone creates a VS Code extension for rahu (hint hint), you would install it from the marketplace and configure:

```json
{
  "rahu.path": "/path/to/rahu"
}
```

## Neovim

### Using nvim-lspconfig

```lua
local lspconfig = require('lspconfig')
local configs = require('lspconfig.configs')

-- Check if rahu is already configured
if not configs.rahu then
  configs.rahu = {
    default_config = {
      cmd = {'rahu'},
      filetypes = {'python'},
      root_dir = lspconfig.util.find_git_ancestor,
      single_file_support = true,
    },
  }
end

lspconfig.rahu.setup{}
```

### Using coc.nvim

Add to your `coc-settings.json`:

```json
{
  "languageserver": {
    "rahu": {
      "command": "rahu",
      "filetypes": ["python"],
      "rootPatterns": [".git", "pyproject.toml", "setup.py"],
      "initializationOptions": {},
      "settings": {}
    }
  }
}
```

## Vim

### Using vim-lsp

```vim
if executable('rahu')
  au User lsp_setup call lsp#register_server({
    \ 'name': 'rahu',
    \ 'cmd': {server_info->['rahu']},
    \ 'allowlist': ['python'],
    \ })
endif
```

### Using ALE

```vim
let g:ale_linters = {
\   'python': ['rahu'],
\}

let g:ale_python_rahu_executable = 'rahu'
let g:ale_python_rahu_options = ''
```

## Emacs

### Using lsp-mode

```elisp
(use-package lsp-mode
  :hook (python-mode . lsp)
  :commands lsp)

;; Register rahu
(with-eval-after-load 'lsp-mode
  (add-to-list 'lsp-language-id-configuration '(python-mode . "python"))
  (lsp-register-client
   (make-lsp-client :new-connection (lsp-stdio-connection "rahu")
                    :activation-fn (lsp-activate-on "python")
                    :server-id 'rahu)))
```

### Using eglot

```elisp
(add-to-list 'eglot-server-programs
             '(python-mode . ("rahu")))
```

## Sublime Text

### Using LSP Package

1. Install the LSP package via Package Control
2. Open Preferences > Package Settings > LSP > Settings
3. Add:

```json
{
  "clients": {
    "rahu": {
      "enabled": true,
      "command": ["rahu"],
      "selector": "source.python",
      "initializationOptions": {}
    }
  }
}
```

## Helix

Add to your `languages.toml`:

```toml
[[language]]
name = "python"
language-servers = ["rahu"]

[language-server.rahu]
command = "rahu"
```

## Testing Your Setup

Create a test Python file:

```python
def greet(name: str) -> str:
    return f"Hello, {name}!"

message = greet("World")
```

Try these features:
- **Hover** over `greet` or `message`
- **Go to definition** on function calls
- **Completion** after typing `message.`
- **Diagnostics** - try introducing an error

## Troubleshooting

If features aren't working:

1. Check the LSP server is running: `ps aux | grep rahu`
2. Verify rahu is in your PATH: `which rahu`
3. Check LSP client logs (varies by editor)
4. See [Troubleshooting Guide](../user-guide/troubleshooting.md)
