#!/bin/sh
set -eu

apk add --no-cache --virtual .headers-more-build \
	gcc libc-dev make openssl-dev pcre2-dev zlib-dev linux-headers curl

version=$(nginx -v 2>&1 | sed -n 's|.*nginx/||p')
test -n "$version"

curl -fsSL "https://nginx.org/download/nginx-${version}.tar.gz" | tar -xz -C /tmp
curl -fsSL "https://github.com/openresty/headers-more-nginx-module/archive/refs/tags/v0.37.tar.gz" | tar -xz -C /tmp

cd "/tmp/nginx-${version}"
./configure --with-compat --add-dynamic-module=/tmp/headers-more-nginx-module-0.37
make modules

mkdir -p /etc/nginx/modules
cp objs/ngx_http_headers_more_filter_module.so /etc/nginx/modules/

apk del .headers-more-build
rm -rf "/tmp/nginx-${version}" /tmp/headers-more-nginx-module-0.37
