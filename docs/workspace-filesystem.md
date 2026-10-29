# Workspace filesystem

The server file service uses the same mounted filesystem implementation as JSH:
[`engine.FS`](../jsh/engine/fs.go). The server-facing layer,
[`server.WorkspaceFileSystem`](../mods/server/filesystem.go), retains file explorer
metadata, filtering, sorting, Git operations, and HTTP response compatibility.
It does not maintain a second filesystem resolver or CRUD backend.

## Configuring mounts

The `--file` flag defines mounts relative to the JSH `/work` workspace:

```sh
machbase-neo serve --file '/=/srv/project,/data=/srv/data'
```

| Configuration | JSH path | Host directory |
| --- | --- | --- |
| `/=/srv/project` | `/work` | `/srv/project` |
| `/data=/srv/data` | `/work/data` | `/srv/data` |

A root mount is implicit when `--file` is specified without one: it uses the
directory containing the running executable. For example,
`--file '/data=/srv/data'` keeps that default root and adds `/work/data`.
Specify `/=<dir>` to replace the default root, as in
`--file '/=/srv/project,/data=/srv/data'`. A single directory,
`--file /srv/project`, still selects the workspace root.

Multiple entries use commas, not semicolons. Relative host paths are resolved
at server configuration time, so child process working directories do not
change their meaning. Commas and equals signs in host paths are not supported.

Mount destinations are always workspace-relative. For example, `/work=/srv/other`
mounts at `/work/work`, not at the JSH system root. A rootless configuration
can still be supplied directly through server configuration; in that case,
implicit workspace parent directories are not mapped to the process working
directory.

## Public paths and runtime visibility

Existing public paths do not acquire a `/work` prefix:

| Surface | Workspace file `data/example.csv` |
| --- | --- |
| Web file API | `/web/api/files/data/example.csv` |
| Database file API | `/db/files/data/example.csv` |
| MCP project path | `/project/data/example.csv` |
| TQL `file()` / HTTP DSL default file directive | `/data/example.csv` |
| JSH | `/work/data/example.csv` |

The file APIs do not expose JSH system mounts such as `/lib` or `/shared`.
An API path `/work/example.csv` means a file named `work/example.csv` inside
the workspace.

Runtime visibility remains intentionally different:

- Default shell/JSH commands receive all configured workspace mounts.
- TQL `SCRIPT()` and services retain root-only visibility.
- Public CGI retains its app-only mount. Its `/work` is an in-memory directory
  that contains only the app mount, so CGI child processes never fall back to
  the server's working directory.
- SFTP remains OS-based and uses the configured root mount as its working
  directory; it is not a mounted VFS endpoint.

Without a root mount, each TQL `SCRIPT()` gets an empty in-memory `/work`.
The service controller also uses an in-memory `/work`, including
`/work/etc/services`; those service configurations disappear on server restart.
Service child processes and TQL `SHELL` commands receive no host `/work` mount
in that case. SFTP requests are rejected rather than falling back to the process
working directory.

## Library ownership

There is no process-global default server filesystem. A server owns its
`server.WorkspaceFileSystem` and passes it to consumers through interfaces:

- `tql.NewLoader(files)` attaches the reader to loaded scripts.
- `task.SetFileSystem(files)` configures inline TQL execution.
- Child tasks and cached execution clones retain the same reader.
- `httpdsl.Execute(content, httpdsl.WithFileSystem(files))` supplies default
  file directives.
- `mdconv.WithFileSystem(files)` supplies file directives inside Markdown HTTP
  fences. Server Markdown RPC and TQL Markdown output propagate this dependency.

The server package owns workspace configuration, file explorer models, and Git
operations. TQL defines its own [`FileSystem`](../mods/tql/tql.go) contract
(`OpenFile`, `Stat`, and `RootDir`); it does not import the server package.
HTTP DSL likewise defines its own file opener interface. The JSH engine remains
independent of these server policies.

HTTP DSL `< /data/payload.json` uses the supplied workspace reader.
`< @/absolute/host/path` remains an explicit OS file operation.

## Compatibility notes

- Duplicate mounts and nested mounts that overlap an existing host entry
  cause server startup to fail. Mount registration order does not select a
  different target. Conflict errors include the requested mount's host directory,
  related parent/child mount mappings, and guidance for resolving the conflict.
  For example, `--file '/backups=/srv/backups'` conflicts if the executable
  directory already contains `backups`. In this case the error explains that the
  root was added implicitly and suggests selecting a different root with
  `--file '/=/srv/workspace,/backups=/srv/backups'`, or moving or renaming the
  conflicting host entry.
- Rootless implicit parent directories can be listed but have no host path.
- Mount points and their ancestors cannot be removed or renamed.
- Server rename continues to reject an existing destination. Host-backed
  moves across mounts use `os.Rename`; cross-device `EXDEV` errors are returned
  without a copy-and-delete fallback.
- File API JSON fields, ordering, hidden-file filtering, and public paths
  remain unchanged.
- Static files under `/public/...` and the non-TQL static fallback of
  `/db/tql/...` are streamed from the workspace without loading the whole file
  into memory. They report `Content-Length` and `Last-Modified`, and honor
  `Range` (206) and `If-Modified-Since` (304) requests. When the extension has
  no explicit server mapping, `Content-Type` falls back to the Go MIME table and
  then to content sniffing.
- The file APIs (`/web/api/files`, `/db/files`) stream file content as well, but always
  return the full current content (200) and ignore conditional and range
  request headers, because `Last-Modified` has one-second resolution and could
  hide an editor save made within the same second. The length is not taken from
  a prior stat, so a file that grows or shrinks concurrently is sent until EOF
  (no snapshot consistency is implied). If the file cannot be opened, stat'ed,
  or read before the response starts, the JSON error `reason` includes the
  cause. A read failure after the response has started aborts the connection
  instead of ending the response normally.
- The public static paths keep a generic `404` without host paths or OS error
  details; unexpected failures other than missing files are logged as warnings.

These mounts are not an OS sandbox. Existing symlink behavior and explicit
OS-path features remain separate from workspace path routing. Mounts are
configured before concurrent use; runtime hot-mount propagation is not provided.
