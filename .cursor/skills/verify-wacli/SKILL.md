---
name: verify-wacli
description: Drive and verify wacli CLI commands, store operations, daemon sync, and IPC socket delegation locally without requiring a live WhatsApp account.
---

# Verify wacli

This skill allows agents to test and verify `wacli` features locally, safely, and deterministically without touching a live WhatsApp account or risking session invalidation.

## 1. Launch

`wacli` is a compiled Go binary (`dist/wacli`) backed by SQLite (`wacli.db` and `session.db`).

### One-Time Build
```bash
pnpm build
```
The build produces `./dist/wacli` with `-tags sqlite_fts5`.

### Isolated Environment Setup
Each verification session must use a private, isolated store directory to prevent lock contention with running daemons:
```bash
STORE="$(mktemp -d /tmp/wacli-verify-XXXXXX)"
```

### Starting Daemon (When testing Follow / IPC Delegation)
If testing background sync or IPC delegation:
```bash
./dist/wacli sync --follow --store "$STORE" &
DAEMON_PID=$!
echo "$DAEMON_PID" > "$STORE/daemon.pid"
# Ready signal: .send.sock or LOCK file present
while [[ ! -e "$STORE/.send.sock" && ! -f "$STORE/LOCK" ]]; do sleep 0.1; done
```

## 2. Doctor

The single read-only diagnostic check that verifies the instance is valid and operable:
```bash
./dist/wacli doctor --store "$STORE" --json
```

**What to check in the doctor output:**
- `"success": true`
- `"fts_enabled": true`
- `"lock_held": false` (or `true` if daemon is intentionally running)
- Database schema is automatically migrated on first access.

## 3. Drive

Drive CLI subcommands by specifying `--store "$STORE"` and `--json` for machine-readable output.

### A. Seed Local Data (No WhatsApp Network Needed)
To test queries, search, or mutations locally:
```bash
sqlite3 "$STORE/wacli.db" <<'SQL'
INSERT OR REPLACE INTO groups (jid, name, owner_jid, created_ts, is_parent, updated_at)
VALUES ('12036302@g.us', 'Finanças', '5511999990001@s.whatsapp.net', 1700000000, 0, 1700000000);

INSERT OR REPLACE INTO group_participants (group_jid, user_jid, role, updated_at)
VALUES
  ('12036302@g.us', '5511999990001@s.whatsapp.net', 'admin', 1700000000),
  ('12036302@g.us', '5511999990002@s.whatsapp.net', 'member', 1700000000);

INSERT OR REPLACE INTO messages (chat_jid, chat_name, msg_id, sender_jid, sender_name, ts, from_me, text, display_text)
VALUES ('12036302@g.us', 'Finanças', 'M1', '5511999990001@s.whatsapp.net', 'Alice', 1700000100, 0, 'Almoço R$ 50', 'Almoço R$ 50');
SQL
```

### B. Drive Commands
1. **Group Participants List:**
   ```bash
   ./dist/wacli groups participants list --store "$STORE" --jid "12036302@g.us" --json
   ```
2. **Messages Search (FTS5):**
   ```bash
   ./dist/wacli messages search --store "$STORE" "Almoço" --json
   ```
3. **Chats Mark-Read (via IPC Delegation):**
   ```bash
   ./dist/wacli chats mark-read --store "$STORE" "12036302@g.us" --json
   ```

## 4. Evidence

All verification runs must capture structured proof in `test-evidence/verify-<feature>-<timestamp>/`:
- `stdout.json`: The exact JSON output returned by the command.
- `stderr.log`: Stderr log (or NDJSON events if `--events` was supplied).
- `exit_code`: Numeric exit code (0 for success).
- `db_dump.sql`: Snapshot of the SQLite tables to prove state mutations.

**Proof Standards:**
- Exit code must be `0`.
- Stderr must not contain unhandled panics or fatal errors.
- Resulting database state must reflect the mutation.

## 5. Cleanup

Always clean up temporary store directories and processes without deleting evidence:
```bash
if [[ -f "$STORE/daemon.pid" ]]; then
  kill "$(cat "$STORE/daemon.pid")" 2>/dev/null || true
fi
rm -rf "$STORE"
```
Evidence in `test-evidence/` is preserved.

## 6. Helpers

The skill includes an automated test harness:
```bash
# Verify doctor diagnostics
.cursor/skills/verify-wacli/scripts/harness.sh doctor

# Prove a specific feature and generate evidence
.cursor/skills/verify-wacli/scripts/harness.sh prove-feature groups-participants
.cursor/skills/verify-wacli/scripts/harness.sh prove-feature messages-search
```
