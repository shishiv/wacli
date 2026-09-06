# Messages Search (FTS5)

## Sub-features
- Full-text SQLite FTS5 search across historical and live WhatsApp messages.
- Filters by chat JID, date ranges, and term matches.
- JSON output for automated ingestion.

## How to get to it (user POV)
The user runs:
```bash
wacli messages search "Almoço" --json
```

## Driving it with harness
Drive the search against a seeded local store:
```bash
.cursor/skills/verify-wacli/scripts/harness.sh prove-feature messages-search
```

**Observable proof:**
- Exit code is `0`.
- Output JSON array contains matching message records with `text`, `chat_jid`, and `msg_id`.
- FTS index triggers populate `messages_fts` automatically upon row insertion into `messages`.

## Gotchas
- Must be compiled with `-tags sqlite_fts5`. `wacli doctor` verifies `fts_enabled: true`.
- FTS match queries sanitize punctuation; exact phrase matching can be done with quotes.
