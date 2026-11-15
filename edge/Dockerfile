FROM node:22-alpine AS ui
WORKDIR /src
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM nginx:1.27-alpine
RUN apk add --no-cache openssl \
	&& rm -f /etc/nginx/templates/default.conf.template
COPY edge/install-headers-more.sh /tmp/install-headers-more.sh
RUN sh /tmp/install-headers-more.sh && rm /tmp/install-headers-more.sh
COPY edge/nginx.main.conf /etc/nginx/nginx.conf
COPY edge/nginx.conf /etc/nginx/conf.d/default.conf
COPY edge/docker-entrypoint.d/40-admin.sh /docker-entrypoint.d/40-admin.sh
RUN chmod +x /docker-entrypoint.d/40-admin.sh
COPY edge/static/ /usr/share/nginx/html/
COPY docs/architecture.md /usr/share/nginx/html/documentation/architecture.md
COPY --from=ui /src/dist/ /usr/share/nginx/html/legacy/
EXPOSE 80
