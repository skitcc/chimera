#!/bin/sh
# Replays the MVP demo scenario with curl. Same steps as e2e/demo_test.go.
# Requires E2E_API_URL and E2E_OUT. curl and jq must be on PATH.
set -eu

api=${E2E_API_URL:?E2E_API_URL is required}
out=${E2E_OUT:?E2E_OUT is required}
api=${api%/}
mkdir -p "$out/responses"
log=$out/curl.log
: > "$log"

suffix=$(od -An -N4 -tx1 /dev/urandom | tr -d ' \n')
artist_email="artist-${suffix}@example.com"
listener_email="listener-${suffix}@example.com"
password=password1
artist_name="DemoArtist${suffix}"
title="Demo Track ${suffix}"
audio=$out/audio.bin
dd if=/dev/zero bs=1024 count=4 of="$audio" 2>/dev/null
size=$(wc -c < "$audio" | tr -d ' ')

step() {
	name=$1
	method=$2
	url=$3
	expect=$4
	token=${5-}
	body=${6-}
	hdr=$out/responses/$name.headers
	resp=$out/responses/$name.body
	if [ -n "$body" ] && [ "$method" != PUT ]; then
		if [ -n "$token" ]; then
			code=$(curl -sS -D "$hdr" -o "$resp" -w '%{http_code}' -X "$method" \
				-H "Authorization: Bearer $token" \
				-H 'Content-Type: application/json' \
				--data-binary @"$body" \
				"$url")
		else
			code=$(curl -sS -D "$hdr" -o "$resp" -w '%{http_code}' -X "$method" \
				-H 'Content-Type: application/json' \
				--data-binary @"$body" \
				"$url")
		fi
	elif [ -n "$body" ]; then
		code=$(curl -sS -D "$hdr" -o "$resp" -w '%{http_code}' -X "$method" \
			--data-binary @"$body" \
			"$url")
	elif [ -n "$token" ]; then
		code=$(curl -sS -D "$hdr" -o "$resp" -w '%{http_code}' -X "$method" \
			-H "Authorization: Bearer $token" \
			"$url")
	else
		code=$(curl -sS -D "$hdr" -o "$resp" -w '%{http_code}' -X "$method" "$url")
	fi
	path=${url#"$api"}
	path=${path%%\?*}
	printf 'STEP %s %s %s -> %s\n' "$name" "$method" "$path" "$code" | tee -a "$log"
	rest=$expect
	ok=0
	while [ -n "$rest" ]; do
		one=${rest%%|*}
		if [ "$code" = "$one" ]; then
			ok=1
			break
		fi
		if [ "$rest" = "$one" ]; then
			break
		fi
		rest=${rest#*|}
	done
	if [ "$ok" -ne 1 ]; then
		printf 'expected %s\n' "$expect" >&2
		cat "$resp" >&2 || true
		exit 1
	fi
}

json_file() {
	jq -n "$@" > "$out/responses/_body.json"
	printf '%s\n' "$out/responses/_body.json"
}

header_value() {
	awk -v key="$1" '
		{
			line = $0
			sub("\r", "", line)
			sep = index(line, ":")
			if (sep == 0) next
			name = substr(line, 1, sep - 1)
			if (tolower(name) == tolower(key)) {
				value = substr(line, sep + 1)
				sub("^[ \t]*", "", value)
				print value
				exit
			}
		}
	' "$2"
}

step 01 GET "$api/ready" 200

body=$(json_file --arg email "$artist_email" --arg password "$password" --arg name "Demo Artist" \
	'{email:$email, password:$password, name:$name}')
step 02 POST "$api/v1/auth/register" 201 "" "$body"
artist_token=$(jq -er .token "$out/responses/02.body")

body=$(json_file --arg email "$artist_email" --arg password "$password" \
	'{email:$email, password:$password}')
step 03 POST "$api/v1/auth/login" 200 "" "$body"
artist_token=$(jq -er .token "$out/responses/03.body")

step 04 GET "$api/v1/me" 200 "$artist_token"
jq -er --arg email "$artist_email" 'select(.email == $email)' "$out/responses/04.body" >/dev/null

body=$(json_file --arg title "$title" --arg artist "$artist_name" --argjson size "$size" \
	'{title:$title, artist:$artist, size:$size}')
step 05 POST "$api/v1/tracks/upload-init" 201 "$artist_token" "$body"
track=$(jq -er .track.id "$out/responses/05.body")
upload=$(jq -er .upload_url "$out/responses/05.body")
jq -er 'select(.track.status == "pending")' "$out/responses/05.body" >/dev/null

step 06 PUT "$upload" '200|204' "" "$audio"

step 07 GET "$api/v1/tracks?limit=100&artist=${artist_name}" 200
jq -er --arg id "$track" 'select(([.items[]?.id] | index($id)) == null)' "$out/responses/07.body" >/dev/null

step 08 POST "$api/v1/tracks/${track}/upload-complete" 200 "$artist_token"
jq -er 'select(.status == "ready")' "$out/responses/08.body" >/dev/null

step 09 GET "$api/v1/tracks?limit=100&artist=${artist_name}" 200
jq -er --arg id "$track" 'select(any(.items[]; .id == $id and .status == "ready"))' "$out/responses/09.body" >/dev/null

body=$(json_file --arg email "$listener_email" --arg password "$password" --arg name "Demo Listener" \
	'{email:$email, password:$password, name:$name}')
step 10 POST "$api/v1/auth/register" 201 "" "$body"
listener_token=$(jq -er .token "$out/responses/10.body")

step 11 POST "$api/v1/tracks/${track}/like" 401
jq -er 'select(.code == "unauthorized")' "$out/responses/11.body" >/dev/null

step 12 POST "$api/v1/tracks/${track}/like" 204 "$listener_token"

step 13 POST "$api/v1/tracks/${track}/like" 409 "$listener_token"
jq -er 'select(.code == "conflict" and .message == "track already liked")' "$out/responses/13.body" >/dev/null

step 14 GET "$api/v1/me/likes?limit=100" 200 "$listener_token"
jq -er --arg id "$track" 'select(any(.items[]; .id == $id))' "$out/responses/14.body" >/dev/null

step 15 GET "$api/v1/tracks/${track}/stream" 302
location=$(header_value Location "$out/responses/15.headers")
if [ -z "$location" ]; then
	echo 'stream response has no Location' >&2
	exit 1
fi

step 16 GET "$location" 200
cmp -s "$audio" "$out/responses/16.body"

step 17 DELETE "$api/v1/tracks/${track}/like" 204 "$listener_token"

step 18 GET "$api/v1/me/likes?limit=100" 200 "$listener_token"
jq -er --arg id "$track" 'select(([.items[]?.id] | index($id)) == null)' "$out/responses/18.body" >/dev/null

printf 'demo requests ok\n' | tee -a "$log"
