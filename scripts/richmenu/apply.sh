#!/bin/sh
# Applies the rich menu in assets/richmenu (docs/open-decisions.md OD-16): validates and
# creates it, uploads its image, makes it the default for every user, then deletes the
# other rich menus created through the Messaging API. A default set through the API
# takes precedence over the one set in LINE Official Account Manager.
#
# The target is whichever channel LINE_MESSAGE_CHANNEL_TOKEN belongs to (the bot's own
# variable); nothing else picks it. The token goes to curl on stdin, not as an argument.
#
#   LINE_MESSAGE_CHANNEL_TOKEN=... sh scripts/richmenu/apply.sh
set -eu

: "${LINE_MESSAGE_CHANNEL_TOKEN:?set LINE_MESSAGE_CHANNEL_TOKEN to the channel access token}"
dir=$(cd "$(dirname "$0")/../../assets/richmenu" && pwd)
api=https://api.line.me/v2/bot

# line METHOD URL [curl options...]: calls the API with the token; fails on an HTTP error.
line() {
	method=$1
	url=$2
	shift 2
	printf 'header = "Authorization: Bearer %s"\n' "$LINE_MESSAGE_CHANNEL_TOKEN" |
		curl -sS --fail-with-body -K - -X "$method" "$url" "$@"
}

line POST "$api/richmenu/validate" -H 'Content-Type: application/json' --data-binary "@$dir/richmenu.json" >/dev/null

id=$(line POST "$api/richmenu" -H 'Content-Type: application/json' --data-binary "@$dir/richmenu.json" |
	sed -n 's/.*"richMenuId": *"\([^"]*\)".*/\1/p')
[ -n "$id" ] || {
	echo "no richMenuId in the response" >&2
	exit 1
}
echo "created $id"

line POST "https://api-data.line.me/v2/bot/richmenu/$id/content" -H 'Content-Type: image/png' --data-binary "@$dir/richmenu.png" >/dev/null
line POST "$api/user/all/richmenu/$id" >/dev/null
echo "default rich menu is now $id"

for old in $(line GET "$api/richmenu/list" | grep -o '"richMenuId": *"[^"]*"' | sed 's/.*"\([^"]*\)"$/\1/'); do
	[ "$old" = "$id" ] && continue
	line DELETE "$api/richmenu/$old" >/dev/null
	echo "deleted $old"
done
