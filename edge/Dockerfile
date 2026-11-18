# Edge nginx: the single entry behind the ingress. Serves the static pages and
# routes /api/v1 to the Gateway, /legacy to web, /mirror, /monitoring and /admin.
# The -otel image ships ngx_otel_module, so every request gets a span.
FROM nginx:1.27-alpine-otel
RUN apk add --no-cache openssl
COPY edge/nginx.otel.conf /etc/nginx/nginx.conf
COPY edge/templates/ /etc/nginx/templates/
COPY edge/docker-entrypoint.d/40-admin.sh /docker-entrypoint.d/40-admin.sh
RUN chmod +x /docker-entrypoint.d/40-admin.sh
COPY edge/static/ /usr/share/nginx/html/
COPY docs/architecture.md /usr/share/nginx/html/documentation/architecture.md
EXPOSE 80
