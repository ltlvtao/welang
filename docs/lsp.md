# The We Language Server Binding

This is the LSP binding document that chapter 21 (toolchain, requirement "What the toolchain does not fix) names: the Language Server Protocol lives here, outside the language specification, and the specification's one promise is repeated as this document's first rule — an editor service's diagnostics are `we check`'s. The same pipeline, the same codes, the same verdicts, whichever face shows them. The language server introduces no diagnostic code of its own; every diagnostic it pushes is the checking pipeline's own, mapped field by field below.

## Protocol baseline

The server speaks the LSP base protocol as specified by the 3.17 revision: JSON-RPC 2.0 messages in the LSP envelope. Two message shapes cross the wire — requests (carry `id`, await a response) and notifications (no `id`, no response) — and the server emits both responses to requests it serves and server-initiated notifications of its own (the diagnostics push).

## Transport

The server communicates over stdio, one process per editor session, and frames each message with the base protocol header:

```
Content-Length: <byte count>\r\n
\r\n
<JSON-RPC message, exactly <byte count> bytes>
```

The header carries the byte length of the UTF-8 JSON body; the reader scans header lines until the empty line, then reads exactly that many bytes. Socket transport is not part of this binding.

## Lifecycle

The session runs one fixed sequence:

1. The client sends the `initialize` request. The server answers with its capabilities — text document synchronization of kind `1` (full: every change carries the whole document text) — and identifies itself as `we`.
2. The client sends the `initialized` notification. Until it arrives (and `initialize` before it), the server serves nothing else: a request received before `initialize` is answered with error `-32002` (server not initialized), save for `exit`, which always lands.
3. Documents open, change, and close (next section); diagnostics flow after each change.
4. The client sends the `shutdown` request; the server answers with a null result and serves no further requests.
5. The client sends the `exit` notification; the process exits — with code 0 when `shutdown` came first, nonzero otherwise.

Error faces, all per the base protocol: malformed JSON in a frame answers `-32700` (parse error); a request for an unknown, non-`$/`-prefixed method answers `-32601` (method not found); notifications with the `$/` prefix are ignored in silence.

## Document synchronization

The client owns the documents; the server sees them through three notifications, with full synchronization (every change carries the whole text — no incremental position edits):

- `textDocument/didOpen` — `{textDocument: {uri, languageId, version, text}}`. The server takes the text as the document's current state.
- `textDocument/didChange` — `{textDocument: {uri, version}, contentChanges: [{text}]}`. The single element's `text` is the whole new document.
- `textDocument/didClose` — `{textDocument: {uri}}`.

After every `didOpen` and every `didChange`, the server checks the document's text through the single-file checking pipeline — the very pipeline `we check <file.we>` runs — and pushes the result as a `textDocument/publishDiagnostics` notification for that URI: an empty array when the text checks clean, the mapped diagnostics otherwise. After `didClose`, the server pushes one final empty array for the URI and forgets the document.

Two disclosed faces of this pipeline choice:

- **Single-file scope.** Each open document is checked as its own single-file compilation, exactly as `we check` checks a named file: `std.` imports resolve from the built-in modules, any other import is the pipeline's `E1302` verdict. The language server does not (yet) walk the project graph or resolve dependencies behind an open document — the same text gets the same verdict from the command line and the editor, which is the promise; project-aware diagnostics are future binding work.
- **Boundary silence.** When the checking pipeline stops at an implementation boundary (a form this reference build does not implement yet — the command line prints its boundary line and exits 70), there is no diagnostic event to report. The server pushes nothing for that revision; the document keeps its previous diagnostics. The boundary is implementation-transient, not a diagnostic.

The advisory findings (the `W` codes, warning severity by default) ride the same push — the single-file pipeline runs them under the warning default, having no manifest to promote them.

## Diagnostic mapping

Each diagnostic of the checking pipeline's JSON face maps onto one LSP `Diagnostic`:

| We diagnostic (JSON Lines face) | LSP `Diagnostic` | Rule |
| --- | --- | --- |
| `severity`: `error` / `warning` / `note` / `help` | `severity`: `1` / `2` / `3` / `4` | Error, Warning, Information, Hint — four levels, one to one |
| `code`: `"E0501"` | `code`: `"E0501"` | The registry code, as-is; the server never mints codes |
| `message` | `message` | The same string — the registry title and the detail |
| `file`, `line`, `column` (1-based) | `range` | `start` = `{line-1, column-1}`, `end` = `{line-1, column}` — LSP positions are 0-based, so the 1-based point becomes a single-character span on the same line |
| `help` | — | No LSP slot; the remediation stays a command-line face |
| — | `source`: `"we"` | Constant |

The notification's envelope carries the document's `uri` and the `version` of the text checked, so an editor never paints diagnostics from a stale revision over fresh text.

## Capability surface and limits

Enabled in this binding: the lifecycle set (`initialize`, `initialized`, `shutdown`, `exit`), full text synchronization (`didOpen`, `didChange`, `didClose`), and the diagnostics push (`publishDiagnostics`). Nothing else: hover, definition, completion, formatting, semantic tokens, code actions, and every other editor capability are not offered — the capabilities object in the `initialize` response says so, and a client asks for nothing the server did not declare. Incremental synchronization, socket transport, and multi-root workspaces are likewise outside this binding. Each addition arrives as a change to this document, in the open, with its mapping rules beside it.
