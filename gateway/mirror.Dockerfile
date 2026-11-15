FROM node:22-alpine AS ui
WORKDIR /src
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
ENV VITE_BASE=/mirror/
ENV VITE_ROUTER_BASENAME=/mirror
ENV VITE_API_URL=/mirror
RUN npm run build

FROM nginx:1.27-alpine
RUN rm -f /etc/nginx/templates/default.conf.template
COPY gateway/install-headers-more.sh /tmp/install-headers-more.sh
RUN sh /tmp/install-headers-more.sh && rm /tmp/install-headers-more.sh
COPY gateway/nginx.main.conf /etc/nginx/nginx.conf
COPY gateway/mirror-nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=ui /src/dist/ /usr/share/nginx/html/mirror/
EXPOSE 80
