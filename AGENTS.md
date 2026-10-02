# Agent notes

Naming contract: [george docs/mcp-naming.md](https://github.com/shotah/george/blob/main/docs/mcp-naming.md).

| Layer | Value |
| --- | --- |
| Server id | `git` |
| Tools | `status_get`, `diff_get`, `commits_list`, `stage_update`, `commit_create` |
| Host-facing | `git__status_get`, … |

Do not put `git` on a tool name. Do not register push. Showing a diff is this server. Applying one is `fs__file_patch`.

Do not `git init` here.
