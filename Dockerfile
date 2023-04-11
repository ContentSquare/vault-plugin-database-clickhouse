FROM hashicorp/vault:1.13.0

RUN mkdir /vault/plugins

ADD clickhouse-database-plugin /vault/plugins

