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

A single directory, `--file /srv/project`, still selects the workspace root.
Multiple entries use commas, not semicolons. Relative host paths are resolved
at server configuration time, so child process working directories do not
change their meaning. Commas and equals signs in host paths are not supported.

Mount destinations are always workspace-relative. For example, `/work=/srv/other`
mounts at `/work/work`, not at the JSH system root. A rootless configuration such
as `--file '/apps/demo=/srv/demo'` provides implicit workspace parent directories
without mapping those parents to the process working directory.

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
  different target.
- Rootless implicit parent directories can be listed but have no host path.
- Mount points and their ancestors cannot be removed or renamed.
- Server rename continues to reject an existing destination. Host-backed
  moves across mounts use `os.Rename`; cross-device `EXDEV` errors are returned
  without a copy-and-delete fallback.
- File API JSON fields, ordering, hidden-file filtering, and public paths
  remain unchanged.

These mounts are not an OS sandbox. Existing symlink behavior and explicit
OS-path features remain separate from workspace path routing. Mounts are
configured before concurrent use; runtime hot-mount propagation is not provided.
