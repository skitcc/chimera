FROM alpine:3.21
RUN apk add --no-cache openssl
COPY keygen.sh /keygen.sh
ENTRYPOINT ["/bin/sh", "/keygen.sh"]
