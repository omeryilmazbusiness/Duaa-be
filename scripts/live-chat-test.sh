#!/bin/sh
# Live Gemini chat check. Requires a running API on :8080.
# Save needs DUA_LIVE_AI_KEY. Persist-only chat: DUA_LIVE_CHAT_ONLY=1.
# The key is never printed or written to the repo.
set -eu
cd "$(dirname "$0")/.."
API="${DUA_API_ORIGIN:-http://localhost:8080}"
echo "══ live Gemini chat ══"
login=$(curl -sS --max-time 10 -X POST "$API/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"email":"duaa@example.com","password":"ChangeMe123!"}')
token=$(python3 -c 'import json,sys; print(json.load(sys.stdin)["token"])' <<EOF
$login
EOF
)
wid=$(python3 -c 'import json,sys; print(json.load(sys.stdin)["session"]["workspace"]["id"])' <<EOF
$login
EOF
)
if [ "${DUA_LIVE_CHAT_ONLY:-}" != "1" ]; then
  if [ -z "${DUA_LIVE_AI_KEY:-}" ]; then
    echo "skip live chat (DUA_LIVE_AI_KEY unset)"
    exit 0
  fi
  save=$(curl -sS --max-time 15 -o /tmp/dua-live-save.json -w "%{http_code}" \
    -X PUT "$API/v1/workspaces/$wid/assistant/settings" \
    -H "Authorization: Bearer $token" \
    -H "Content-Type: application/json" \
    -d "{\"provider\":\"gemini\",\"model\":\"gemini-3.1-flash-lite\",\"apiKey\":\"$DUA_LIVE_AI_KEY\",\"dailyLimit\":30,\"enabled\":true}")
  echo "save=$save"
  python3 -c 'import json; d=json.load(open("/tmp/dua-live-save.json")); e=d.get("error") or {}; print("save_model=", d.get("model"), "configured=", d.get("configured"), "hint=", d.get("keyHint"), "err=", e.get("code",""), e.get("message","")[:160])'
  if [ "$save" != "200" ]; then
    echo "FAIL save"
    exit 1
  fi
fi
chat=$(curl -sS --max-time 20 -o /tmp/dua-live-chat.json -w "%{http_code}" \
  -X POST "$API/v1/workspaces/$wid/assistant/chat" \
  -H "Authorization: Bearer $token" \
  -H "Content-Type: application/json" \
  -d '{"locale":"tr","messages":[{"role":"user","content":"Tek kelime: tamam"}]}')
echo "chat=$chat"
python3 -c 'import json; d=json.load(open("/tmp/dua-live-chat.json")); e=d.get("error") or {}; print("reply=", (d.get("reply") or "")[:160] or "-", "err=", e.get("code",""), e.get("message","")[:200])'
if [ "$chat" != "200" ]; then
  echo "FAIL chat"
  exit 1
fi
python3 - <<'PY'
import json
d=json.load(open("/tmp/dua-live-chat.json"))
if not (d.get("reply") or "").strip():
    raise SystemExit("FAIL empty reply")
print("OK live chat")
PY
