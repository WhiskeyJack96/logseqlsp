## A LSP implementation for LogSeq flavored markdown

- Attempts to improve the cli editing experience of markdown files that have log seq embed and queries by support hover, go to definition, and select references
- If you have ideas for additional features please let me know :)

## Usage

- Download the release or build from source and copy the binary into your path. Then configure your lsp integration with the binary name.
- For example in helix add this to `~/.config/helix/languages.toml`
  - ```yaml
        [[language]]
        name = "markdown"
        scope = "source.md"
        injection-regex = "md|markdown"
        file-types = ["md", "markdown"]
        language-server = { command = "logseqlsp", args=[] }
        indent = { tab-width = 2, unit = "  " }
    ```

## Planned features
  - Support for code actions to do the following will be hit in the next pass
    - Rotate between todo, doing, done
    - Create page if it does not exist
  - Support for autocomplete on tags, properties, links will be added after code actions
  - Refactor/Rename page/tag/property might be possible
  - Tree Sitter syntax file may be added (help appreciated)
  - Virtual text for neovim will likely require an nvim plugin (help appreciate)
