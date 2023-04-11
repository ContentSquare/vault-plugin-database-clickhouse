package vault_plugin_database_clickhouse

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/hashicorp/go-secure-stdlib/strutil"
	dbplugin "github.com/hashicorp/vault/sdk/database/dbplugin/v5"
	"github.com/hashicorp/vault/sdk/database/helper/dbutil"
	"github.com/hashicorp/vault/sdk/helper/template"
	"github.com/hashicorp/vault/sdk/logical"
)

const (
	defaultClickhouseRevocationStmts = `
		DROP USER IF EXISTS '{{name}}';
	`

	defaultClickhouseRotateCredentialsSQL = `
		ALTER USER IF EXISTS '{{name}}' IDENTIFIED BY '{{password}}';
	`
	clickhouseTypeName = "clickhouse"

	DefaultUserNameTemplate = `{{ printf "v-%s-%s-%s-%s" (.DisplayName | truncate 10) (.RoleName | truncate 10) (random 20) (unix_time) | truncate 32 }}`
)

var (
	_ dbplugin.Database       = (*Clickhouse)(nil)
	_ logical.PluginVersioner = (*Clickhouse)(nil)
)

type Clickhouse struct {
	*clickhouseConnectionProducer

	usernameProducer        template.StringTemplate
	defaultUsernameTemplate string

	version string
}

func (c *Clickhouse) PluginVersion() logical.PluginVersion {
	return logical.PluginVersion{
		Version: c.version,
	}
}

// New implements builtinplugins.BuiltinFactory
func New(defaultUsernameTemplate string, version string) func() (interface{}, error) {
	return func() (interface{}, error) {
		if defaultUsernameTemplate == "" {
			return nil, fmt.Errorf("missing default username template")
		}
		db := newClickhouse(defaultUsernameTemplate)
		// Wrap the plugin with middleware to sanitize errors
		dbType := dbplugin.NewDatabaseErrorSanitizerMiddleware(db, db.SecretValues)

		db.version = version
		return dbType, nil
	}
}

func newClickhouse(defaultUsernameTemplate string) *Clickhouse {
	connProducer := &clickhouseConnectionProducer{}

	return &Clickhouse{
		clickhouseConnectionProducer: connProducer,
		defaultUsernameTemplate:      defaultUsernameTemplate,
	}
}

func (c *Clickhouse) Type() (string, error) {
	return clickhouseTypeName, nil
}

func (c *Clickhouse) getConnection(ctx context.Context) (*sql.DB, error) {
	db, err := c.Connection(ctx)
	if err != nil {
		return nil, err
	}

	return db.(*sql.DB), nil
}

func (c *Clickhouse) Initialize(ctx context.Context, req dbplugin.InitializeRequest) (dbplugin.InitializeResponse, error) {
	usernameTemplate, err := strutil.GetString(req.Config, "username_template")
	if err != nil {
		return dbplugin.InitializeResponse{}, err
	}

	if usernameTemplate == "" {
		usernameTemplate = c.defaultUsernameTemplate
	}

	up, err := template.NewTemplate(template.Template(usernameTemplate))
	if err != nil {
		return dbplugin.InitializeResponse{}, fmt.Errorf("unable to initialize username template: %w", err)
	}

	c.usernameProducer = up

	_, err = c.usernameProducer.Generate(dbplugin.UsernameMetadata{})
	if err != nil {
		return dbplugin.InitializeResponse{}, fmt.Errorf("invalid username template: %w", err)
	}

	err = c.clickhouseConnectionProducer.Initialize(ctx, req.Config, req.VerifyConnection)
	if err != nil {
		return dbplugin.InitializeResponse{}, err
	}

	resp := dbplugin.InitializeResponse{
		Config: req.Config,
	}

	return resp, nil
}

func (c *Clickhouse) NewUser(ctx context.Context, req dbplugin.NewUserRequest) (dbplugin.NewUserResponse, error) {
	if len(req.Statements.Commands) == 0 {
		return dbplugin.NewUserResponse{}, dbutil.ErrEmptyCreationStatement
	}

	username, err := c.usernameProducer.Generate(req.UsernameConfig)
	if err != nil {
		return dbplugin.NewUserResponse{}, err
	}

	password := req.Password

	expirationStr := req.Expiration.Format("2006-01-02 15:04:05-0700")

	queryMap := map[string]string{
		"name":       username,
		"username":   username,
		"password":   password,
		"expiration": expirationStr,
	}

	if err := c.executeStatementsWithMap(ctx, req.Statements.Commands, queryMap); err != nil {
		return dbplugin.NewUserResponse{}, err
	}

	resp := dbplugin.NewUserResponse{
		Username: username,
	}
	return resp, nil
}

func (c *Clickhouse) DeleteUser(ctx context.Context, req dbplugin.DeleteUserRequest) (dbplugin.DeleteUserResponse, error) {
	revocationStmts := req.Statements.Commands
	if len(revocationStmts) == 0 {
		revocationStmts = []string{defaultClickhouseRevocationStmts}
	}

	queryMap := map[string]string{
		"name":     req.Username,
		"username": req.Username,
	}
	if err := c.executeStatementsWithMap(ctx, revocationStmts, queryMap); err != nil {
		return dbplugin.DeleteUserResponse{}, err
	}
	return dbplugin.DeleteUserResponse{}, nil
}

func (c *Clickhouse) UpdateUser(ctx context.Context, req dbplugin.UpdateUserRequest) (dbplugin.UpdateUserResponse, error) {
	if req.Password == nil && req.Expiration == nil {
		return dbplugin.UpdateUserResponse{}, fmt.Errorf("no change requested")
	}

	if req.Password != nil {
		rotateStatments := req.Password.Statements.Commands
		if len(rotateStatments) == 0 {
			rotateStatments = []string{defaultClickhouseRotateCredentialsSQL}
		}

		queryMap := map[string]string{
			"name":     req.Username,
			"username": req.Username,
			"password": req.Password.NewPassword,
		}

		if err := c.executeStatementsWithMap(ctx, rotateStatments, queryMap); err != nil {
			return dbplugin.UpdateUserResponse{}, err
		}
	}

	// Expiration change/update is currently a no-op

	return dbplugin.UpdateUserResponse{}, nil
}

// executeStatementsWithMap loops through the given templated SQL statements and
// applies the map to them, interpolating values into the templates, returning
// the resulting username and password
func (c *Clickhouse) executeStatementsWithMap(ctx context.Context, statements []string, queryMap map[string]string) error {
	// Grab the lock
	c.Lock()
	defer c.Unlock()

	// Get the connection
	db, err := c.getConnection(ctx)
	if err != nil {
		return err
	}
	// Execute the statements
	for _, stmt := range statements {
		for _, query := range strutil.ParseArbitraryStringSlice(stmt, ";") {
			query = strings.TrimSpace(query)
			if len(query) == 0 {
				continue
			}
			query = dbutil.QueryHelper(query, queryMap)
			if _, err = db.ExecContext(ctx, query); err != nil {
				return fmt.Errorf("unable to execute query. err=%v", err.Error())
			}
		}
	}
	return nil
}
