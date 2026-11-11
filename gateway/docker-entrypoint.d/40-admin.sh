#!/bin/sh
set -eu

out=/etc/nginx/admin.location

if [ -z "${ADMIN_USER:-}" ] || [ -z "${ADMIN_PASSWORD:-}" ]; then
	cat >"$out" <<'EOF'
location = /admin {
	return 404;
}

location /admin/ {
	return 404;
}
EOF
	exit 0
fi

hash=$(openssl passwd -apr1 "$ADMIN_PASSWORD")
printf '%s:%s\n' "$ADMIN_USER" "$hash" >/etc/nginx/admin.htpasswd
chmod 644 /etc/nginx/admin.htpasswd

cat >"$out" <<'EOF'
location = /admin {
	return 301 /admin/;
}

location /admin/ {
	auth_basic "Chimera database";
	auth_basic_user_file /etc/nginx/admin.htpasswd;
	proxy_pass http://adminer:8080/;
	proxy_http_version 1.1;
	proxy_set_header Host $host;
	proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
	proxy_set_header X-Forwarded-Prefix /admin;
	proxy_set_header Accept-Encoding "";
	proxy_redirect / /admin/;
	sub_filter_once off;
	sub_filter_types text/html text/css;
	sub_filter 'action="/' 'action="/admin/';
	sub_filter "action='/" "action='/admin/";
	sub_filter 'href="/' 'href="/admin/';
	sub_filter 'src="/' 'src="/admin/';
}
EOF
