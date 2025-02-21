# Clickhouse Database Secret Engine

This plugin provides clickhouse connectivity for clickhouse database using SQL user management

Checkout [Docker Hub](https://hub.docker.com/r/contentsquareplatform/vault-plugin-database-clickhouse) for a docker image that embeds the pluging, versioned by current version of Vault

* [vault 1.11](https://hub.docker.com/r/contentsquareplatform/vault-plugin-database-clickhouse/tags?page=1&name=1.11)
* [vault 1.12](https://hub.docker.com/r/contentsquareplatform/vault-plugin-database-clickhouse/tags?page=1&name=1.12)
* [vault 1.13](https://hub.docker.com/r/contentsquareplatform/vault-plugin-database-clickhouse/tags?page=1&name=1.13)
* [vault 1.14](https://hub.docker.com/r/contentsquareplatform/vault-plugin-database-clickhouse/tags?page=1&name=1.14)
* [vault 1.15](https://hub.docker.com/r/contentsquareplatform/vault-plugin-database-clickhouse/tags?page=1&name=1.15)
* [vault 1.16](https://hub.docker.com/r/contentsquareplatform/vault-plugin-database-clickhouse/tags?page=1&name=1.16)
* [vault 1.17](https://hub.docker.com/r/contentsquareplatform/vault-plugin-database-clickhouse/tags?page=1&name=1.17)
* [vault 1.18](https://hub.docker.com/r/contentsquareplatform/vault-plugin-database-clickhouse/tags?page=1&name=1.18)

# Plugin Sha256 (linux amd64 binary)

| Version | Sha256                                                          |
|---------|:----------------------------------------------------------------|
| 0.1.0   | 6537135bdb3fab24ceb97a4f3d68308428558d75d1c67ef716790730485e8fce|
| 0.1.1   | 18c7795c17db236b06351af89ea4d4f0dcbefa71ab9f56073be007ee5ccf3ae7|
| 0.1.2   | 00fd995d848c0702f2f78151ebfb2724a0f94b88443a8362534e533ed1578b25|
| 0.1.3   | e4c9039b5dc221728d146c807ba891be2dc92782e4c148a3eda37333b6642379|
| 0.1.4   | 4571b9709fd3cc9e7c7d4f561fa4ba6541fb1f827b2a59081162b342553a3b0e|


# Build & Tests

make test will run the whole test suite

```bash
~# make test
```

make build will run build the corresponding plugin for the current os/arch

```bash
~# make build
```

## Installation

The Vault plugin system is documented on the [Vault documentation site](https://www.vaultproject.io/docs/internals/plugins.html).

You will need to define a plugin directory using the `plugin_directory` configuration directive, then place the
`vault-plugin-database-clickhouse` executable generated above, into the directory.

**Please note:** Versions v0.2.0 onwards of this plugin are incompatible with Vault versions before 1.6.0 due to an update of the database plugin interface.

Sample commands for registering and starting to use the plugin:

note: sha256 could be found in the release page, per tag. Download the targz, and run the sha command against the right binary. 

```bash
~# SHA256=$(shasum -a 256 plugins/clickhouse-database-plugin | cut -d' ' -f1)

~# vault secrets enable database

~# vault plugin register -sha256=$SHA256 database clickhouse-database-plugin
```

At this stage you are now ready to initialize the plugin to connect to clickhouse cluster using unencrypted or encrypted communications.

Prior to initializing the plugin, ensure that you have created an administration account. Vault will use the user specified here to create/update/revoke database credentials. That user must have the appropriate permissions to perform actions upon other database users.


## Pre requisites

Clickhouse user management must be switched to SQL as follows

eg. file /etc/clickhouse-server/users.d/vaultadminuser.xml

```xml

<clickhouse>
    <users>
        <vault_user>
            <profile>default</profile>
            <quota>default</quota>
            <password_sha256_hex>65e84be33532fb784c48129675f9eff3a682b27168c0ea744b2cf58ee02337c5</password_sha256_hex>
            <access_management>1</access_management>
        </vault_user>
    </users>
</clickhouse>
```

Also, roles must be defined in advance as per the vault roles.

```sql
CREATE ROLE readonly ON CLUSTER '{cluster_name}' SETTINGS max_execution_time=30, max_concurrent_queries_for_user=30, max_threads=8, max_query_size=50485760, max_memory_usage=32819380224, max_memory_usage_for_user=33356251136, max_ast_elements=50000000, distributed_product_mode='local', log_queries=1, distributed_group_by_no_merge=1, optimize_move_to_prewhere=0, readonly=2, optimize_min_equality_disjunction_chain_length=100;
```

## Cluster creation statements

When dealing with a clickhouse cluster, (multiple replicas/shards) we may use the `ON CLUSTER` statement

Creation statement

```bash
creation_statements="CREATE USER '{{name}}' IDENTIFIED BY '{{password}}' ON CLUSTER 'my_cluster'; GRANT readonly TO '{{name}}' ON CLUSTER 'my_cluster';SET DEFAULT ROLE readonly TO '{{name}}'" 
```

## Default statements

Default revocation statements:

```sql
DROP USER IF EXISTS '{{name}}';
```

Default Rotate credential statement

```sql
ALTER USER IF EXISTS '{{name}}' IDENTIFIED BY '{{password}}';
```

Default username template

```bash
`{{ printf "v-%s-%s-%s-%s" (.DisplayName | truncate 10) (.RoleName | truncate 10) (random 20) (unix_time) | truncate 32 }}`
```

## Plugin configuration

| Setting         | Description                                         | Type | default value |
|-----------------|:----------------------------------------------------|-----:|---------------|
| tls             | TLS secure connection to clickhouse                 | bool | false         |
| tls_skip_verify | Whether to check certificate CA upon TLS connection | bool | true          |

## Running a dev vault

```bash 
~# docker run -p 8200:8200 -it contentsquareplatform/vault-plugin-database-clickhouse:1.13.1-latest server -dev -dev-plugin-dir=/vault/plugins -dev-root-token-id=bladibla
...
2023-04-14T15:32:04.888Z [INFO]  identity: entities restored
2023-04-14T15:32:04.888Z [INFO]  identity: groups restored
2023-04-14T15:32:04.888Z [INFO]  core: post-unseal setup complete
2023-04-14T15:32:04.888Z [INFO]  core: vault is unsealed
2023-04-14T15:32:04.890Z [INFO]  expiration: revoked lease: lease_id=auth/token/root/ha010bb14be140f3cdf07b143e150b6cdc75f822e4ac925f65b106553363ccdb2
2023-04-14T15:32:04.891Z [INFO]  core: successful mount: namespace="" path=secret/ type=kv version=""
WARNING! dev mode is enabled! In this mode, Vault runs entirely in-memory
and starts unsealed with a single unseal key. The root token is already
authenticated to the CLI, so you can immediately begin using Vault.

You may need to set the following environment variables:

    $ export VAULT_ADDR='http://0.0.0.0:8200'

The unseal key and root token are displayed below in case you want to
seal/unseal the Vault or re-authenticate.

Unseal Key: iOhLjm7fixKtDJbMicErCveh0A7GB/6i+4UstXG0Udo=
Root Token: bladibla

The following dev plugins are registered in the catalog:
    - clickhouse-database-plugin    << The clickhouse plugin has been loaded.

Development mode should NOT be used in production installations!
```

```bash
~# export VAULT_ADDR=http://localhost:8200
~# vault login 
Token (will be hidden): ******** 
Success! You are now authenticated. The token information displayed below
is already stored in the token helper. You do NOT need to run "vault login"
again. Future Vault requests will automatically use this token.

Key                  Value
---                  -----
token                bladibla
token_accessor       EgHmgtCbYhCEIxmTUV2RiQYn
token_duration       ∞
token_renewable      false
token_policies       ["root"]
identity_policies    []
policies             ["root"]
```

Check the plugin & version

```bash
~# vault plugin list database          
Name                                 Version
----                                 -------
cassandra-database-plugin            v1.13.1+builtin.vault
clickhouse-database-plugin           v0.1.0
couchbase-database-plugin            v0.9.0+builtin
elasticsearch-database-plugin        v0.13.1+builtin
hana-database-plugin                 v1.13.1+builtin.vault
influxdb-database-plugin             v1.13.1+builtin.vault
mongodb-database-plugin              v1.13.1+builtin.vault
mongodbatlas-database-plugin         v0.9.0+builtin
mssql-database-plugin                v1.13.1+builtin.vault
mysql-aurora-database-plugin         v1.13.1+builtin.vault
mysql-database-plugin                v1.13.1+builtin.vault
mysql-legacy-database-plugin         v1.13.1+builtin.vault
mysql-rds-database-plugin            v1.13.1+builtin.vault
postgresql-database-plugin           v1.13.1+builtin.vault
redis-database-plugin                v0.2.0+builtin
redis-elasticache-database-plugin    v0.2.0+builtin
redshift-database-plugin             v1.13.1+builtin.vault
snowflake-database-plugin            v0.7.0+builtin
```

## Configure the clickhouse plugin

Enable vault database mount path

```bash 
~# vault secrets enable database
Success! Enabled the database secrets engine at: database/

```

Create a connection to your clickhouse deployment

```bash
~# vault write database/config/my-clickhouse \
    plugin_name=clickhouse-database-plugin  \
    plugin_version=v0.1.0 \
    allowed_roles="readonly" \
    connection_url="clickhouse://my-clickhouse-server:9000?username={{username}}&password={{password}}" \
    username="vault_user" \
    password="mySEcreTP@assw0Rd" \
    username_template="{{.DisplayName}}-{{.RoleName}}-{{unix_time}}-{{random 8}}"
Success! Data written to: database/config/my-clickhouse
```

Create a vault role to use the db connection

```bash
~# vault write database/roles/readonly \
    creation_statements="CREATE USER '{{name}}' IDENTIFIED BY '{{password}}'; GRANT readonly TO '{{name}}';SET DEFAULT ROLE readonly TO '{{name}}'" \
    revocation_statements="REVOKE readonly FROM '{{name}}'; DROP USER '{{name}}';" \
    default_ttl="1h" \
    db_name="my-clickhouse"
Success! Data written to: database/roles/readonly
```

Request a database token for role readonly

```bash
vault read database/creds/readonly 

Key                Value
---                -----
lease_id           database/creds/readonly/sYXjvvB50ZjXtygRQepYZtTr
lease_duration     1h
lease_renewable    true
password           dTbla2n-0talksjf4uK-rWJ
username           token-readonly-1681487166-1hAOaK6V
```

Check on clickhouse side

```bash
clickhouse-server:9000|default :) SHOW USERS

SHOW USERS

Query id: 15553c58-9b60-41ce-a19a-f0ff38b35cd8

┌─name───────────────────────────────┐
│ default                            │
│ token-readonly-1681487166-1hAOaK6V │
│ vault_users                        │
└────────────────────────────────────┘

3 rows in set. Elapsed: 0.001 sec.

clickhouse-server:9000|default :)
```
