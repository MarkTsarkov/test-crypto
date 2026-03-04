FROM alpine:3.15

RUN apk update && \
    apk upgrade && \
    apk add bash && \
    rm -rf /var/cache/apk/*

add https://github.com/pressly/goose/releases/download/v3.14.0/goose_linux_x86_64 /bin/goose
RUN chmod +x /bin/goose

ARG MIGRATION_DIR

WORKDIR /root

ADD pkg/postgres/migrations/*.sql .
ADD pkg/postgres/migrator.sh .
ADD .env .

RUN chmod +x migrator.sh

ENTRYPOINT ["./migrator.sh"]