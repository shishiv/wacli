#!/usr/bin/env bash
set -euo pipefail

# wacli verification harness
# Drives isolated local instances without touching a live WhatsApp account.

REPO_ROOT="$(git -C "$(dirname "${BASH_SOURCE[0]}")" rev-parse --show-toplevel)"
BIN="$REPO_ROOT/dist/wacli"
EVIDENCE_BASE="$REPO_ROOT/test-evidence"

action="${1:-help}"

ensure_build() {
  if [[ ! -x "$BIN" ]]; then
    echo "==> Building dist/wacli..."
    (cd "$REPO_ROOT" && pnpm build)
  fi
}

new_store() {
  local store
  store="$(mktemp -d /tmp/wacli-verify-XXXXXX)"
  echo "$store"
}

init_store() {
  local store="$1"
  ensure_build
  # Running doctor automatically runs schema migrations on wacli.db
  "$BIN" doctor --store "$store" --json >/dev/null 2>&1 || true
}

seed_store() {
  local store="$1"
  local db="$store/wacli.db"
  init_store "$store"

  sqlite3 "$db" <<'SQL'
-- Seed groups
INSERT OR REPLACE INTO groups (jid, name, owner_jid, created_ts, is_parent, updated_at)
VALUES ('12036302@g.us', 'Test Finance', '5511999990001@s.whatsapp.net', 1700000000, 0, 1700000000);

-- Seed group participants
INSERT OR REPLACE INTO group_participants (group_jid, user_jid, role, updated_at)
VALUES
  ('12036302@g.us', '5511999990001@s.whatsapp.net', 'admin', 1700000000),
  ('12036302@g.us', '5511999990002@s.whatsapp.net', 'member', 1700000000);

-- Seed contacts
INSERT OR REPLACE INTO contacts (jid, phone, push_name, full_name, updated_at)
VALUES
  ('5511999990001@s.whatsapp.net', '5511999990001', 'Alice', 'Alice Silva', 1700000000),
  ('5511999990002@s.whatsapp.net', '5511999990002', 'Bob', 'Bob Souza', 1700000000);

-- Seed chats
INSERT OR REPLACE INTO chats (jid, kind, name, last_message_ts, unread, unread_count)
VALUES
  ('12036302@g.us', 'group', 'Test Finance', 1700000100, 1, 1),
  ('5511999990002@s.whatsapp.net', 'dm', 'Bob Souza', 1700000050, 0, 0);

-- Seed messages
INSERT OR REPLACE INTO messages (chat_jid, chat_name, msg_id, sender_jid, sender_name, ts, from_me, text, display_text)
VALUES
  ('12036302@g.us', 'Test Finance', 'MSG-001', '5511999990001@s.whatsapp.net', 'Alice Silva', 1700000100, 0, 'Almoço R$ 50', 'Almoço R$ 50');
SQL
}

case "$action" in
  doctor)
    store="$(new_store)"
    trap 'rm -rf "$store"' EXIT
    init_store "$store"
    "$BIN" doctor --store "$store" --json
    ;;

  seed)
    store="$(new_store)"
    seed_store "$store"
    echo "STORE=$store"
    ;;

  run-with-seed)
    shift
    store="$(new_store)"
    trap 'rm -rf "$store"' EXIT
    seed_store "$store"
    "$BIN" --store "$store" "$@"
    ;;

  prove-feature)
    feature="${2:-groups-participants}"
    timestamp="$(date +%Y%m%d-%H%M%S)"
    evidence_dir="$EVIDENCE_BASE/verify-$feature-$timestamp"
    mkdir -p "$evidence_dir"

    store="$(new_store)"
    trap 'rm -rf "$store"' EXIT
    seed_store "$store"

    case "$feature" in
      groups-participants)
        cmd=("$BIN" groups participants list --store "$store" --jid "12036302@g.us" --json)
        ;;
      doctor)
        cmd=("$BIN" doctor --store "$store" --json)
        ;;
      messages-search)
        cmd=("$BIN" messages search --store "$store" "Almoço" --json)
        ;;
      *)
        echo "Unknown feature: $feature" >&2
        exit 1
        ;;
    esac

    echo "==> Driving command: ${cmd[*]}"
    set +e
    "${cmd[@]}" > "$evidence_dir/stdout.json" 2> "$evidence_dir/stderr.log"
    exit_code=$?
    set -e

    echo "$exit_code" > "$evidence_dir/exit_code"
    sqlite3 "$store/wacli.db" ".dump" > "$evidence_dir/db_dump.sql"

    echo "==> Evidence captured in: $evidence_dir"
    cat "$evidence_dir/stdout.json"
    echo ""
    echo "Exit code: $exit_code"
    ;;

  *)
    echo "Usage: $0 {doctor|seed|run-with-seed <wacli args>|prove-feature <name>}"
    exit 1
    ;;
esac
