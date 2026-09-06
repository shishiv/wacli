# Chats Mark-Read IPC Delegation

## Sub-features
- Mark a chat as read or unread.
- When running standalone, acquires store lock and writes app state.
- When the background sync daemon holds the store lock, automatically routes request over `.send.sock` Unix socket without failing due to lock contention.

## How to get to it (user POV)
The user or a bot running concurrently with `wacli sync --follow` runs:
```bash
wacli chats mark-read 12036302@g.us
```
Or to mark unread:
```bash
wacli chats mark-unread 12036302@g.us
```

## Driving it with harness
1. Start the follow daemon in an isolated store:
   ```bash
   STORE="$(mktemp -d /tmp/wacli-verify-XXXXXX)"
   # Wait for .send.sock
   ./dist/wacli sync --follow --store "$STORE" &
   DAEMON_PID=$!
   ```
2. Execute mark-read command:
   ```bash
   ./dist/wacli chats mark-read --store "$STORE" "12036302@g.us" --json
   ```
3. Observable proof:
   - Output contains `{"chat": "12036302@g.us", "read": true, "delegated": true}` or equivalent.
   - Command exits with code 0 instead of `store is locked by another process`.

## Gotchas
- Requires `.send.sock` to exist in the store directory if the daemon holds the lock.
- If no daemon is running and no lock is held, it executes directly via normal store write path.
