# Doctor Diagnostics

## Sub-features
- Validates local store directory permissions (0700/0600).
- Checks SQLite database presence and automatically applies schema migrations.
- Detects if FTS5 is compiled into the binary.
- Verifies lock status (`LOCK` file and flock).
- Reports authentication status (`session.db` device presence).

## How to get to it (user POV)
The user runs:
```bash
wacli doctor
```
Or for machine-readable status:
```bash
wacli doctor --json
```

## Driving it with harness
```bash
.cursor/skills/verify-wacli/scripts/harness.sh doctor
```

**Observable proof:**
- Output returns a JSON object with:
  - `"success": true`
  - `"fts_enabled": true`
  - `"store_dir": "<path>"`
  - `"lock_held": false`
  - `"store": {"messages": 0, "chats": 0, ...}`
- Database files `wacli.db` and `session.db` exist in the store dir.

## Gotchas
- If the store directory does not exist, `doctor` creates it with secure owner-only permissions (`0700`).
- If another process holds the lock, `"lock_held": true` is reported without blocking.
