FROM node:22-alpine AS ui
WORKDIR /src
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM nginx:1.27-alpine
RUN apk add --no-cache openssl \
	&& rm -f /etc/nginx/templates/default.conf.template
COPY gateway/nginx.conf /etc/nginx/conf.d/default.conf
COPY gateway/docker-entrypoint.d/40-admin.sh /docker-entrypoint.d/40-admin.sh
RUN chmod +x /docker-entrypoint.d/40-admin.sh
COPY gateway/static/ /usr/share/nginx/html/
COPY README.md /usr/share/nginx/html/documentation/README.md
COPY --from=ui /src/dist/ /usr/share/nginx/html/legacy/
EXPOSE 80
