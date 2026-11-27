#!/bin/sh
# Source after load_env. Every private compose run gets fresh credentials.
# Optional E2E_ENV_OUTPUT appends overrides for a subsequent compose invocation.
for secret_name in POSTGRES_PASSWORD IDENTITY_DB_PASSWORD CATALOG_DB_PASSWORD SOCIAL_DB_PASSWORD MINIO_ROOT_PASSWORD AUTH_CODE_HMAC_KEY; do
 secret_value=$(od -An -N32 -tx1 /dev/urandom | tr -d ' \n')
 [ "${#secret_value}" -eq 64 ] || exit 1
 export "$secret_name=$secret_value"
 if [ "${GITHUB_ACTIONS:-}" = true ]; then printf '::add-mask::%s\n' "$secret_value"; fi
 if [ -n "${E2E_ENV_OUTPUT:-}" ]; then printf '%s=%s\n' "$secret_name" "$secret_value" >> "$E2E_ENV_OUTPUT"; fi
done
export S3_ACCESS_KEY=${MINIO_ROOT_USER:-chimera}
export S3_SECRET_KEY=$MINIO_ROOT_PASSWORD
if [ -n "${E2E_ENV_OUTPUT:-}" ]; then printf 'S3_ACCESS_KEY=%s\n' "$S3_ACCESS_KEY" >> "$E2E_ENV_OUTPUT"; printf 'S3_SECRET_KEY=%s\n' "$S3_SECRET_KEY" >> "$E2E_ENV_OUTPUT"; fi
unset secret_name secret_value
