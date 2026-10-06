# Editor integrations

Every editor integration is a thin client of `awf lsp`. Validation and rendering live in the server. An editor needs an LSP client and one custom request.

## Server

Command: `awf lsp`. Transport: LSP JSON-RPC over stdio.

The server handles OpenAWF documents:

- Workflows: `.yaml` files with a top-level `openawf` key.
- Tasks: `.md` files whose YAML frontmatter has an `openawf` key.

The server ignores every other file. A client registers it for all YAML and Markdown files.

## Features

- `textDocument/publishDiagnostics`: validation errors for the open document.
- `textDocument/definition`: jumps from a `$ref` value to its target, when the server advertises `definitionProvider`.
- `awf/render`: a custom request that renders the current in-memory document as a full HTML page.

## `awf/render`

Request params:

```json
{ "uri": "file:///path/to/workflow.yaml" }
```

Result:

```json
{ "html": "<!DOCTYPE html>..." }
```

The server renders the unsaved buffer content it holds from `textDocument/didChange`. A non-OpenAWF document returns a JSON-RPC error. A preview client sends the request after each change and displays `html` in a browser view with scripts enabled.

## Neovim

```lua
local configs = require("lspconfig.configs")
local lspconfig = require("lspconfig")

if not configs.awf then
  configs.awf = {
    default_config = {
      cmd = { "awf", "lsp" },
      filetypes = { "yaml", "markdown" },
      root_dir = lspconfig.util.root_pattern(".awf", ".git"),
      single_file_support = true,
    },
  }
end

lspconfig.awf.setup({})
```

Request a render from Neovim:

```lua
vim.lsp.buf_request(0, "awf/render", { uri = vim.uri_from_bufnr(0) }, function(err, result)
  if err then return vim.notify(err.message, vim.log.levels.ERROR) end
  local path = vim.fn.tempname() .. ".html"
  vim.fn.writefile(vim.split(result.html, "\n"), path)
  vim.ui.open(path)
end)
```

## VS Code

The extension in `vscode/` starts `awf lsp` and adds the `AWF: Open Preview` command. See `vscode/README.md`.
