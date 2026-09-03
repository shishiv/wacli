# Groups Participants List

## Sub-features
- List all participants of a group from the local SQLite database.
- Output formatting in human table and structured JSON (`--json`).
- Displays user JID, role (`admin`, `superadmin`, `member`), and updated timestamp.

## How to get to it (user POV)
The user runs:
```bash
wacli groups participants list --jid 12036302@g.us
```
Or for automation scripts like `shishiv/gastei`:
```bash
wacli groups participants list --jid 12036302@g.us --json
```

## Driving it with harness
Using the verification harness with seeded SQLite data:
```bash
.cursor/skills/verify-wacli/scripts/harness.sh prove-feature groups-participants
```
Or directly:
```bash
STORE="$(mktemp -d /tmp/wacli-verify-XXXXXX)"
# 1. Init store & seed
.cursor/skills/verify-wacli/scripts/harness.sh seed
# 2. Query
./dist/wacli groups participants list --store "$STORE" --jid "12036302@g.us" --json
```

**Observable proof:**
- Exit code is `0`.
- Output is a JSON array containing participant objects with `user_jid` and `role`.
- Matches the rows inserted into `group_participants` in `wacli.db`.

## Gotchas
- The group JID must end with `@g.us`.
- If the group is not yet synced in local SQLite, the list will be empty (`[]`). Run `wacli sync` to populate.
