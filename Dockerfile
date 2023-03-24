FROM hashicorp/vault:1.13.0

RUN mkdir /vault/plugins

ADD vault-plugin-database-clickhouse /vault/plugins

