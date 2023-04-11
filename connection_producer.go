package vault_plugin_database_clickhouse

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/hashicorp/go-secure-stdlib/parseutil"
	"github.com/hashicorp/vault/sdk/database/helper/connutil"
	"github.com/mitchellh/mapstructure"
)

// clickhouseConnectionProducer implements ConnectionProducer and provides a generic producer for most sql databases
type clickhouseConnectionProducer struct {
	ConnectionURL      string `json:"connection_url"          mapstructure:"connection_url"          structs:"connection_url"`
	MaxOpenConnections int    `json:"max_open_connections"    mapstructure:"max_open_connections"    structs:"max_open_connections"`
	MaxIdleConnections int    `json:"max_idle_connections"    mapstructure:"max_idle_connections"    structs:"max_idle_connections"`

	MaxConnectionLifetimeRaw interface{} `json:"max_connection_lifetime" mapstructure:"max_connection_lifetime" structs:"max_connection_lifetime"`
	Username                 string      `json:"username" mapstructure:"username" structs:"username"`
	Password                 string      `json:"password" mapstructure:"password" structs:"password"`

	TLS           bool `json:"tls" mapstructure:"tls" structs:"tls"`
	TLSSkipVerify bool `json:"tls_skip_verify" mapstructure:"tls_skip_verify" structs:"tls_skip_verify"`

	// https://github.com/ClickHouse/clickhouse-go#dsn
	Database string `json:"database" mapstructure:"database" structs:"database"`
	Debug    bool   `json:"debug" mapstructure:"debug" structs:"debug"`

	RawConfig             map[string]interface{}
	maxConnectionLifetime time.Duration
	Initialized           bool
	db                    *sql.DB
	sync.Mutex
}

func (c *clickhouseConnectionProducer) Initialize(ctx context.Context, conf map[string]interface{}, verifyConnection bool) error {
	_, err := c.Init(ctx, conf, verifyConnection)
	return err
}

func (c *clickhouseConnectionProducer) Init(ctx context.Context, conf map[string]interface{}, verifyConnection bool) (map[string]interface{}, error) {
	c.Lock()
	defer c.Unlock()
	c.RawConfig = conf

	err := mapstructure.WeakDecode(conf, &c)
	if err != nil {
		return nil, err
	}

	if len(c.ConnectionURL) == 0 {
		return nil, fmt.Errorf("connection_url cannot be empty")
	}

	// ConnBuilder
	connBuilder, err := NewConnStringBuilderFromConnString(c.ConnectionURL)
	if err != nil {
		return nil, err
	}
	if c.TLS {
		connBuilder.WithTLS(c.TLSSkipVerify)
	}
	if c.Database != "" {
		connBuilder.WithDatabase(c.Database)
	}
	if c.Username != "" {
		connBuilder.WithUsername(c.Username)
	}
	if c.Password != "" {
		connBuilder.WithPassword(c.Password)
	}
	c.ConnectionURL, err = connBuilder.BuildConnectionString()
	if err != nil {
		return nil, err
	}

	if c.MaxOpenConnections == 0 {
		c.MaxOpenConnections = 4
	}

	if c.MaxIdleConnections == 0 {
		c.MaxIdleConnections = c.MaxOpenConnections
	}
	if c.MaxIdleConnections > c.MaxOpenConnections {
		c.MaxIdleConnections = c.MaxOpenConnections
	}
	if c.MaxConnectionLifetimeRaw == nil {
		c.MaxConnectionLifetimeRaw = "0s"
	}

	c.maxConnectionLifetime, err = parseutil.ParseDurationSecond(c.MaxConnectionLifetimeRaw)
	if err != nil {
		return nil, fmt.Errorf("invalid max_connection_lifetime: %w", err)
	}

	// Set initialized to true at this point since all fields are set,
	// and the connection can be established at a later time.
	c.Initialized = true

	if verifyConnection {
		if _, err = c.Connection(ctx); err != nil {
			return nil, fmt.Errorf("error verifying - connection: %w", err)
		}

		if err = c.db.PingContext(ctx); err != nil {
			return nil, fmt.Errorf("error verifying - ping: %w", err)
		}
	}

	return c.RawConfig, nil
}

func (c *clickhouseConnectionProducer) Connection(ctx context.Context) (interface{}, error) {
	if !c.Initialized {
		return nil, connutil.ErrNotInitialized
	}

	// If we already have a DB, test it and return
	if c.db != nil {
		if err := c.db.PingContext(ctx); err == nil {
			return c.db, nil
		}
		// If the ping was unsuccessful, close it and ignore errors as we'll be
		// reestablishing anyways
		c.db.Close()
	}
	var err error
	c.db, err = sql.Open("clickhouse", c.ConnectionURL)
	if err != nil {
		return nil, err
	}

	// Set some connection pool settings. We don't need much of this,
	// since the request rate shouldn't be high.
	c.db.SetMaxOpenConns(c.MaxOpenConnections)
	c.db.SetMaxIdleConns(c.MaxIdleConnections)
	c.db.SetConnMaxLifetime(c.maxConnectionLifetime)

	return c.db, nil
}

func (c *clickhouseConnectionProducer) SecretValues() map[string]string {
	return map[string]string{
		c.Password: "[password]",
	}
}

// Close attempts to close the connection
func (c *clickhouseConnectionProducer) Close() error {
	// Grab the write lock
	c.Lock()
	defer c.Unlock()

	if c.db != nil {
		c.db.Close()
	}

	c.db = nil

	return nil
}

type connStringBuilder struct {
	host          string
	port          int
	database      string
	debug         bool
	tls           bool
	tlsSkipVerify bool
	username      string
	password      string
	extra         map[string]string
}

func (c *connStringBuilder) WithHost(host string) *connStringBuilder {
	c.host = host
	return c
}

func (c *connStringBuilder) WithPort(port int) *connStringBuilder {
	c.port = port
	return c
}

func (c *connStringBuilder) WithDatabase(database string) *connStringBuilder {
	c.database = database
	return c
}

func (c *connStringBuilder) WithTLS(skipVerify bool) *connStringBuilder {
	c.tls = true
	c.tlsSkipVerify = skipVerify
	return c
}

func (c *connStringBuilder) WithDebug() *connStringBuilder {
	c.debug = true
	return c
}

func (c *connStringBuilder) WithUsername(username string) *connStringBuilder {
	c.username = username
	return c
}

func (c *connStringBuilder) WithPassword(password string) *connStringBuilder {
	c.password = password
	return c
}

func NewConnStringBuilderFromConnString(connString string) (*connStringBuilder, error) {
	c := &connStringBuilder{
		extra: map[string]string{},
	}
	parsed, err := url.Parse(connString)
	if err != nil {
		return nil, fmt.Errorf("error parsing url. err=%v", err.Error())
	}
	split := strings.Split(parsed.Host, ":")
	c.host = split[0]
	if c.port, err = strconv.Atoi(split[1]); err != nil {
		return nil, fmt.Errorf("unable to parse port. got=%s. err=%v", split[1], err.Error())
	}
	c.database = strings.Replace(parsed.Path, "/", "", -1)
	for k, v := range parsed.Query() {
		switch k {
		case "debug":
			debug, err := strconv.ParseBool(v[0])
			if err != nil {
				return nil, err
			}
			c.debug = debug
		case "username":
			c.username = v[0]
		case "password":
			c.password = v[0]
		case "secure":
			secure, err := strconv.ParseBool(v[0])
			if err != nil {
				return nil, err
			}
			c.tls = secure
		case "skip_verify":
			skipVerify, err := strconv.ParseBool(v[0])
			if err != nil {
				return nil, err
			}
			c.tlsSkipVerify = skipVerify
		default:
			c.extra[k] = v[0]
		}
	}
	return c, nil
}

func (c *connStringBuilder) BuildConnectionString() (string, error) {
	err := c.Check()
	if err != nil {
		return "", err
	}
	host := fmt.Sprintf("%s:%d", c.host, c.port)

	q := make(url.Values)

	if c.tls {
		q.Set("secure", "true")
	}
	if c.tlsSkipVerify {
		q.Set("skip_verify", "true")
	}
	if c.username != "" {
		q.Set("username", c.username)
	}
	if c.password != "" {
		q.Set("password", c.password)
	}
	if c.debug {
		q.Set("debug", "true")
	}
	for k, v := range c.extra {
		q.Set(k, v)
	}
	dsn := (&url.URL{
		Scheme:   "tcp",
		Host:     host,
		RawQuery: q.Encode(),
		Path:     c.database,
	}).String()
	return dsn, nil
}

func (c *connStringBuilder) Check() error {
	var errors []error
	if c.host == "" {
		errors = append(errors, fmt.Errorf("host is missing"))
	}
	if c.port == 0 {
		errors = append(errors, fmt.Errorf("port is missing"))
	}
	if len(errors) > 0 {
		return fmt.Errorf("check errors: %v", errors)
	}
	return nil
}
