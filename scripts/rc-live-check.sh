#!/usr/bin/env bash
set -euo pipefail

: "${GPT_MIRROR_BASE_URL:?set GPT_MIRROR_BASE_URL, e.g. http://127.0.0.1:9000}"
: "${GPT_MIRROR_ADMIN_PASSWORD:?set GPT_MIRROR_ADMIN_PASSWORD}"
: "${GPT_MIRROR_ACCOUNT_ID:?set GPT_MIRROR_ACCOUNT_ID}"

base="${GPT_MIRROR_BASE_URL%/}"
account_id="$GPT_MIRROR_ACCOUNT_ID"
model="${GPT_MIRROR_MODEL:-auto}"
totp="${GPT_MIRROR_TOTP:-}"
conversation_id=""
workdir="$(mktemp -d)"

cleanup() {
  if [[ -n "$conversation_id" && -n "${token:-}" ]]; then
    curl --fail --silent --show-error \
      -H "Authorization: Bearer $token" \
      -H 'Content-Type: application/json' \
      -d "$(jq -nc --argjson accountId "$account_id" --arg conversationId "$conversation_id" '{accountId:$accountId,conversationId:$conversationId}')" \
      "$base/api/chatgpt/conversations/delete" >/dev/null 2>&1 || true
  fi
  rm -rf "$workdir"
}
trap cleanup EXIT

require_success() {
  local payload="$1"
  local label="$2"
  local status
  status="$(jq -r '.status // -1' <<<"$payload")"
  if [[ "$status" != "0" ]]; then
    local message
    message="$(jq -r '.message // "unknown error"' <<<"$payload")"
    printf 'FAIL %-24s status=%s message=%s\n' "$label" "$status" "$message" >&2
    exit 1
  fi
}

api_post() {
  local path="$1"
  local data="$2"
  curl --fail --silent --show-error \
    -H "Authorization: Bearer $token" \
    -H 'Content-Type: application/json' \
    -d "$data" \
    "$base$path"
}

printf 'GPT Mirror RC live check\n'
printf 'base=%s accountId=%s\n' "$base" "$account_id"

login_payload="$(jq -nc --arg password "$GPT_MIRROR_ADMIN_PASSWORD" --arg validateCode "$totp" '{password:$password,validateCode:$validateCode}')"
login_response="$(curl --fail --silent --show-error -H 'Content-Type: application/json' -d "$login_payload" "$base/api/login")"
require_success "$login_response" 'admin login'
token="$(jq -r '.data.accessToken // empty' <<<"$login_response")"
if [[ -z "$token" ]]; then
  echo 'FAIL admin login: access token missing' >&2
  exit 1
fi
printf 'PASS %-24s\n' 'admin login'

health_response="$(api_post '/api/chatgpt/health' "$(jq -nc --argjson accountId "$account_id" '{accountId:$accountId}')")"
require_success "$health_response" 'account health'
health_state="$(jq -r '.data.state // "unknown"' <<<"$health_response")"
printf 'PASS %-24s state=%s\n' 'account health' "$health_state"

models_response="$(api_post '/api/chatgpt/models' "$(jq -nc --argjson accountId "$account_id" '{accountId:$accountId}')")"
require_success "$models_response" 'model list'
model_count="$(jq '.data | length' <<<"$models_response")"
if [[ "$model_count" -lt 1 ]]; then
  echo 'FAIL model list: no models returned' >&2
  exit 1
fi
if [[ "$model" == "auto" ]]; then
  candidate="$(jq -r '.data[] | select(.slug == "auto" or .id == "auto") | (.slug // .id)' <<<"$models_response" | head -n1)"
  if [[ -n "$candidate" ]]; then
    model="$candidate"
  else
    model="$(jq -r '.data[0] | (.slug // .id)' <<<"$models_response")"
  fi
fi
printf 'PASS %-24s count=%s model=%s\n' 'model list' "$model_count" "$model"

history_response="$(api_post '/api/chatgpt/conversations/list' "$(jq -nc --argjson accountId "$account_id" '{accountId:$accountId,cursor:"",limit:10}')")"
require_success "$history_response" 'history list'
history_count="$(jq '.data.items | length' <<<"$history_response")"
printf 'PASS %-24s count=%s\n' 'history list' "$history_count"

marker="gpt-mirror-rc-$(date -u +%Y%m%dT%H%M%SZ)-$$"
create_body="$(jq -nc --argjson accountId "$account_id" --arg model "$model" --arg message "RC check $marker. Reply with exactly: RC-OK" '{accountId:$accountId,model:$model,message:$message,temporary:false}')"
create_stream="$workdir/create.sse"

curl --fail --silent --show-error --no-buffer \
  -H "Authorization: Bearer $token" \
  -H 'Content-Type: application/json' \
  -d "$create_body" \
  "$base/api/chatgpt/conversations/create" >"$create_stream"

conversation_id="$(sed -n 's/^data:[[:space:]]*//p' "$create_stream" | jq -r 'select(.conversationId != null and .conversationId != "") | .conversationId' | tail -n1)"
if [[ -z "$conversation_id" ]]; then
  echo 'FAIL create conversation: stream did not yield conversationId' >&2
  exit 1
fi
if ! grep -q '^event: done' "$create_stream"; then
  echo 'FAIL create conversation: stream did not complete' >&2
  exit 1
fi
printf 'PASS %-24s conversationId=%s\n' 'create conversation' "$conversation_id"

conversation_response="$(api_post '/api/chatgpt/conversations/get' "$(jq -nc --argjson accountId "$account_id" --arg conversationId "$conversation_id" '{accountId:$accountId,conversationId:$conversationId}')")"
require_success "$conversation_response" 'get created conversation'
returned_id="$(jq -r '.data.id // empty' <<<"$conversation_response")"
if [[ "$returned_id" != "$conversation_id" ]]; then
  echo 'FAIL get created conversation: conversation ID mismatch' >&2
  exit 1
fi
parent_id="$(jq -r '.data.messages[-1].id // empty' <<<"$conversation_response")"
if [[ -z "$parent_id" ]]; then
  echo 'FAIL get created conversation: parent message ID missing' >&2
  exit 1
fi
printf 'PASS %-24s\n' 'get created conversation'

found=0
for _ in $(seq 1 10); do
  history_response="$(api_post '/api/chatgpt/conversations/list' "$(jq -nc --argjson accountId "$account_id" '{accountId:$accountId,cursor:"",limit:50}')")"
  require_success "$history_response" 'created history visibility'
  if jq -e --arg id "$conversation_id" '.data.items[]? | select(.id == $id)' <<<"$history_response" >/dev/null; then
    found=1
    break
  fi
  sleep 1
done
if [[ "$found" != "1" ]]; then
  echo 'FAIL created history visibility: conversation not found in upstream history' >&2
  exit 1
fi
printf 'PASS %-24s\n' 'created history visibility'

continue_body="$(jq -nc --argjson accountId "$account_id" --arg conversationId "$conversation_id" --arg model "$model" --arg parentMessageId "$parent_id" --arg message "Continue RC check $marker. Reply with exactly: RC-CONTINUE-OK" '{accountId:$accountId,conversationId:$conversationId,model:$model,parentMessageId:$parentMessageId,message:$message,temporary:false}')"
continue_stream="$workdir/continue.sse"
curl --fail --silent --show-error --no-buffer \
  -H "Authorization: Bearer $token" \
  -H 'Content-Type: application/json' \
  -d "$continue_body" \
  "$base/api/chatgpt/conversations/continue" >"$continue_stream"
continued_id="$(sed -n 's/^data:[[:space:]]*//p' "$continue_stream" | jq -r 'select(.conversationId != null and .conversationId != "") | .conversationId' | tail -n1)"
if [[ "$continued_id" != "$conversation_id" ]] || ! grep -q '^event: done' "$continue_stream"; then
  echo 'FAIL continue conversation: stream did not preserve and complete the conversation' >&2
  exit 1
fi
printf 'PASS %-24s conversationId=%s\n' 'continue conversation' "$conversation_id"

rename_response="$(api_post '/api/chatgpt/conversations/rename' "$(jq -nc --argjson accountId "$account_id" --arg conversationId "$conversation_id" --arg title "GPT Mirror RC $marker" '{accountId:$accountId,conversationId:$conversationId,title:$title}')")"
require_success "$rename_response" 'rename conversation'
printf 'PASS %-24s\n' 'rename conversation'

archive_response="$(api_post '/api/chatgpt/conversations/archive' "$(jq -nc --argjson accountId "$account_id" --arg conversationId "$conversation_id" '{accountId:$accountId,conversationId:$conversationId,archived:true}')")"
require_success "$archive_response" 'archive conversation'
unarchive_response="$(api_post '/api/chatgpt/conversations/archive' "$(jq -nc --argjson accountId "$account_id" --arg conversationId "$conversation_id" '{accountId:$accountId,conversationId:$conversationId,archived:false}')")"
require_success "$unarchive_response" 'unarchive conversation'
printf 'PASS %-24s\n' 'archive round trip'

delete_response="$(api_post '/api/chatgpt/conversations/delete' "$(jq -nc --argjson accountId "$account_id" --arg conversationId "$conversation_id" '{accountId:$accountId,conversationId:$conversationId}')")"
require_success "$delete_response" 'delete conversation'
printf 'PASS %-24s\n' 'delete conversation'
conversation_id=""

echo 'PASS live provider RC check completed'
