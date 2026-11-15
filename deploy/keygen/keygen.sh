#!/bin/sh
# Generates the local Ed25519 JWT key pair once. The private key goes to a volume
# only Identity mounts; Gateway gets the public half only.
set -eu

private=/keys/private/jwt.pem
public=/keys/public/jwt.pub
owner=${KEY_OWNER_UID:?KEY_OWNER_UID is required}

if [ ! -s "$private" ]; then
	umask 077
	openssl genpkey -algorithm ed25519 -out "$private"
fi
openssl pkey -in "$private" -pubout -out "$public"
chown "$owner" "$private"
chmod 0400 "$private"
chmod 0444 "$public"
