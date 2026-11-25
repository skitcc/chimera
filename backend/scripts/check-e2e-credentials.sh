#!/bin/sh
# Validate without printing credentials. Local runs can use generated passwords;
# GitHub Actions must receive both repository secrets explicitly.
set -eu
E2E_USER_PASSWORD=${E2E_USER_PASSWORD:-}
E2E_NEW_PASSWORD=${E2E_NEW_PASSWORD:-}
LC_ALL=C
export LC_ALL
if [ "${GITHUB_ACTIONS:-}" != true ] && [ -z "${E2E_USER_PASSWORD:-}${E2E_NEW_PASSWORD:-}" ]; then
 exit 0
fi
cr=$(printf '\r')
case "$E2E_USER_PASSWORD$E2E_NEW_PASSWORD" in
 *"$cr"*|*'
'*) printf '%s\n' 'E2E passwords must be single-line values' >&2; exit 1 ;;
esac
if [ "${#E2E_USER_PASSWORD}" -lt 8 ] || [ "${#E2E_USER_PASSWORD}" -gt 72 ] || \
   [ "${#E2E_NEW_PASSWORD}" -lt 8 ] || [ "${#E2E_NEW_PASSWORD}" -gt 72 ] || \
   [ "$E2E_USER_PASSWORD" = "$E2E_NEW_PASSWORD" ]; then
 printf '%s\n' 'Configure distinct E2E_USER_PASSWORD and E2E_NEW_PASSWORD (8-72 bytes) in GitHub Actions Secrets or local environment' >&2
 exit 1
fi
if [ "${GITHUB_ACTIONS:-}" = true ]; then
 printf '::add-mask::%s\n' "$E2E_USER_PASSWORD"
 printf '::add-mask::%s\n' "$E2E_NEW_PASSWORD"
fi
