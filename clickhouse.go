package vault_plugin_database_clickhouse

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp/go-secure-stdlib/strutil"
	dbplugin "github.com/hashicorp/vault/sdk/database/dbplugin/v5"
	"github.com/hashicorp/vault/sdk/database/helper/dbutil"
	"github.com/hashicorp/vault/sdk/helper/template"
)

const (
	defaultClickhouseRevocationStmts = `
		DROP USER '{{name}}';
	`

	defaultClickhouseRotateCredentialsSQL = `
		ALTER USER '{{name}}' IDENTIFIED BY '{{password}}';
	`
	clickhouseTypeName = "clickhouse"

	DefaultUserNameTemplate = `{{ printf "v-%s-%s-%s-%s" (.DisplayName | truncate 10) (.RoleName | truncate 10) (random 20) (unix_time) | truncate 32 }}`
)

var _ dbplugin.Database = (*Clickhouse)(nil)

type Clickhouse struct {
	*clickhouseConnectionProducer

	usernameProducer        template.StringTemplate
	defaultUsernameTemplate string
}

// New implements builtinplugins.BuiltinFactory
func New(defaultUsernameTemplate string) func() (interface{}, error) {
	return func() (interface{}, error) {
		if defaultUsernameTemplate == "" {
			return nil, fmt.Errorf("missing default username template")
		}
		db := newClickhouse(defaultUsernameTemplate)
		// Wrap the plugin with middleware to sanitize errors
		dbType := dbplugin.NewDatabaseErrorSanitizerMiddleware(db, db.SecretValues)

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

	if err := c.executePreparedStatementsWithMap(ctx, req.Statements.Commands, queryMap); err != nil {
		return dbplugin.NewUserResponse{}, err
	}

	resp := dbplugin.NewUserResponse{
		Username: username,
	}
	return resp, nil
}

func (c *Clickhouse) DeleteUser(ctx context.Context, req dbplugin.DeleteUserRequest) (dbplugin.DeleteUserResponse, error) {
	// Grab the read lock
	c.Lock()
	defer c.Unlock()

	// Get the connection
	db, err := c.getConnection(ctx)
	if err != nil {
		return dbplugin.DeleteUserResponse{}, err
	}

	revocationStmts := req.Statements.Commands
	// Use a default SQL statement for revocation if one cannot be fetched from the role
	if len(revocationStmts) == 0 {
		revocationStmts = []string{defaultClickhouseRevocationStmts}
	}

	// Start a transaction
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return dbplugin.DeleteUserResponse{}, err
	}
	defer tx.Rollback()

	for _, stmt := range revocationStmts {
		for _, query := range strutil.ParseArbitraryStringSlice(stmt, ";") {
			query = strings.TrimSpace(query)
			if len(query) == 0 {
				continue
			}

			query = strings.ReplaceAll(query, "{{name}}", req.Username)
			query = strings.ReplaceAll(query, "{{username}}", req.Username)
			_, err = tx.ExecContext(ctx, query)
			if err != nil {
				return dbplugin.DeleteUserResponse{}, err
			}
		}
	}

	// Commit the transaction
	err = tx.Commit()
	return dbplugin.DeleteUserResponse{}, err
}

func (c *Clickhouse) UpdateUser(ctx context.Context, req dbplugin.UpdateUserRequest) (dbplugin.UpdateUserResponse, error) {
	if req.Password == nil && req.Expiration == nil {
		return dbplugin.UpdateUserResponse{}, fmt.Errorf("no change requested")
	}

	if req.Password != nil {
		err := c.changeUserPassword(ctx, req.Username, req.Password.NewPassword, req.Password.Statements.Commands)
		if err != nil {
			return dbplugin.UpdateUserResponse{}, fmt.Errorf("failed to change password: %w", err)
		}
	}

	// Expiration change/update is currently a no-op

	return dbplugin.UpdateUserResponse{}, nil
}

func (c *Clickhouse) changeUserPassword(ctx context.Context, username, password string, rotateStatements []string) error {
	if username == "" || password == "" {
		return errors.New("must provide both username and password")
	}

	if len(rotateStatements) == 0 {
		rotateStatements = []string{defaultClickhouseRotateCredentialsSQL}
	}

	queryMap := map[string]string{
		"name":     username,
		"username": username,
		"password": password,
	}

	if err := c.executePreparedStatementsWithMap(ctx, rotateStatements, queryMap); err != nil {
		return err
	}
	return nil
}

// executePreparedStatementsWithMap loops through the given templated SQL statements and
// applies the map to them, interpolating values into the templates, returning
// the resulting username and password
func (c *Clickhouse) executePreparedStatementsWithMap(ctx context.Context, statements []string, queryMap map[string]string) error {
	// Grab the lock
	c.Lock()
	defer c.Unlock()

	// Get the connection
	db, err := c.getConnection(ctx)
	if err != nil {
		return err
	}
	// Start a transaction
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	// Execute each query
	for _, stmt := range statements {
		for _, query := range strutil.ParseArbitraryStringSlice(stmt, ";") {
			query = strings.TrimSpace(query)
			if len(query) == 0 {
				continue
			}

			query = dbutil.QueryHelper(query, queryMap)

			stmt, _ := tx.PrepareContext(ctx, query)
			if _, err := stmt.ExecContext(ctx); err != nil {
				stmt.Close()
				return err
			}
			stmt.Close()
		}
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}
