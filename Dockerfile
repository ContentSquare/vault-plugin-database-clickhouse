ARG VAULT_VERSION
FROM hashicorp/vault:${VAULT_VERSION}

RUN mkdir /vault/plugins

ADD clickhouse-database-plugin /vault/plugins

