# proposal-platform

Shared HTTP contracts and infrastructure; this module has no business transitions
and no executable. Both independent programs import it through a local Go module
replacement. It never imports either program.

## Architecture

| Package/type | Definition and ownership |
| --- | --- |
| `api` | The package owns the version-one HTTP value contract. |
| `api.Delivery` | A delivery owns an ID and candidate input. |
| `api.Candidate` | Candidate material owns identity, immutable hunks, Doubts and documents. |
| `api.Source` | A source owns repository path and two pinned commits. |
| `api.Anchor` | An anchor locates an exact changed hunk or a file-wide finding. |
| `api.Hunk` | A hunk owns path, header, content and must-sign classification. |
| `api.Doubt` | A Doubt owns a role finding, its rendered document, anchors and Worker disposition. |
| `api.Document` | A document owns text and immutable server-rendered HTML. |
| `api.Decision` | A decision owns one committed user effect and frozen verdict evidence. |
| `api.Feed` | A feed page owns journal identity, cursor and ordered decisions. |
| `api.Receipt` | A receipt owns committed sequence, declared keys and navigation target. |
| `api.Frame` | A frame owns epoch, cursor and keyed replacements. |
| `journal` | Storage owns atomic append, lookup, ordered replay and exclusive ownership. |
| `journal.Journal` | A journal owns a SQLite connection and process lock. |
| `journal.Record` | A record owns an opaque application event and command digest. |
| `syncview` | Synchronization owns keyed projections and bounded delta retention. |
| `syncview.Store` | A store owns serialized values, boot epoch and long-poll wakeups. |
| `httpjson` | HTTP helpers own strict decoding, JSON responses and same-origin mutation checks. |

Journal interface: `Open`, `Lookup`, `Append`, `Read`, `Close`; the application
serializes transitions and the journal never computes them. Each append is one
FULL-synchronized SQLite transaction. An indexed proposal column gives logical
per-proposal streams; a global sequence provides feed order. Schema version 1 is
strict; there is no compatibility decoder.

View interface: `Publish(sequence,puts)`, `Read(epoch,after,prefixes)`,
`Wait(ctx,epoch,after,prefixes,duration)`. Only supplied keys are changed. The
browser replaces values and resets the selected cache on epoch mismatch or
expired cursor. A changed subscription starts with a reset request.

## Storage and platform choices

The database uses `github.com/mattn/go-sqlite3` (a C compiler is required). WAL and
synchronous FULL commit before acknowledgement. The checksum covers both event
payload and feed payload; startup checks SQLite integrity and replay version.
SQLite owns crash-safe file writes. A Unix flock holds single-process projection
ownership for the journal lifetime. This implementation targets macOS/Linux.

The global event table uses proposal indexes for logical streams; workflow feed
transactions may span multiple proposals. No separate Markdown, snapshot file,
feed checkpoint file or browser action log can get out of step with a committed
application event. Application services apply owned values from committed bytes; delivery reuses its
already-owned immutable candidate and indexes. Callers cannot mutate stored state
through returned pointers.

Each program has its own database and each browser has its own cache. Browser
synchronization IDs are boot epochs; decision feed identity is durable. No schemas
or protocols from the old Go/Rust programs are accepted.

`api.Highlight` carries immutable server-rendered before/after syntax HTML on a
hunk. A hunk also carries its exact patch and old/new starting lines for Peek and
navigation. These presentation fields are excluded from the signature identity.
