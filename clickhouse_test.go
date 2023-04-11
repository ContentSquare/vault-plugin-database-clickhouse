package vault_plugin_database_clickhouse

import (
	"context"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/hashicorp/vault/sdk/database/dbplugin/v5"
	dbtesting "github.com/hashicorp/vault/sdk/database/dbplugin/v5/testing"
	"github.com/stretchr/testify/require"
	clickhousehelper "github.com/vfoucault/vault-plugin-database-clickhouse/testhelpers/clickhouse"
)

var _ dbplugin.Database = (*Clickhouse)(nil)

func TestClickhouse_Initialize(t *testing.T) {
	type testCase struct {
		adminUser     string
		adminPassword string
	}

	tests := map[string]testCase{
		"non-special characters in root password": {
			adminUser:     "admin_local",
			adminPassword: "B44a30c4C04D0aAaE140",
		},
		"special characters in root password": {
			adminUser:     "admin_local",
			adminPassword: "#secret!%25#{@}",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			testInitialize(t, test.adminUser, test.adminPassword)
		})
	}
}

func testInitialize(t *testing.T, adminUser, adminPassword string) {
	cleanup, connURL := clickhousehelper.PrepareTestContainer(t, false, adminUser, adminPassword)

	defer cleanup()

	parsedClickhouseConfig, _ := url.Parse(connURL)
	tmplConnURL := fmt.Sprintf("tcp://%s?username={{username}}&password={{password}}", parsedClickhouseConfig.Host)

	type testCase struct {
		initRequest  dbplugin.InitializeRequest
		expectedResp dbplugin.InitializeResponse

		expectErr         bool
		expectInitialized bool
	}

	tests := map[string]testCase{
		"missing connection_url": {
			initRequest: dbplugin.InitializeRequest{
				Config:           map[string]interface{}{},
				VerifyConnection: true,
			},
			expectedResp:      dbplugin.InitializeResponse{},
			expectErr:         true,
			expectInitialized: false,
		},
		"basic config": {
			initRequest: dbplugin.InitializeRequest{
				Config: map[string]interface{}{
					"connection_url": connURL,
				},
				VerifyConnection: true,
			},
			expectedResp: dbplugin.InitializeResponse{
				Config: map[string]interface{}{
					"connection_url": connURL,
				},
			},
			expectErr:         false,
			expectInitialized: true,
		},
		"username and password replacement in connection_url": {
			initRequest: dbplugin.InitializeRequest{
				Config: map[string]interface{}{
					"connection_url": tmplConnURL,
					"username":       adminUser,
					"password":       adminPassword,
				},
				VerifyConnection: true,
			},
			expectedResp: dbplugin.InitializeResponse{
				Config: map[string]interface{}{
					"connection_url": tmplConnURL,
					"username":       adminUser,
					"password":       adminPassword,
				},
			},
			expectErr:         false,
			expectInitialized: true,
		},
		"invalid username template": {
			initRequest: dbplugin.InitializeRequest{
				Config: map[string]interface{}{
					"connection_url":    connURL,
					"username_template": "{{.FieldThatDoesNotExist}}",
				},
				VerifyConnection: true,
			},
			expectedResp:      dbplugin.InitializeResponse{},
			expectErr:         true,
			expectInitialized: false,
		},
		"bad username template": {
			initRequest: dbplugin.InitializeRequest{
				Config: map[string]interface{}{
					"connection_url":    connURL,
					"username_template": "{{ .DisplayName", // Explicitly bad template
				},
				VerifyConnection: true,
			},
			expectedResp:      dbplugin.InitializeResponse{},
			expectErr:         true,
			expectInitialized: false,
		},
		"custom username template": {
			initRequest: dbplugin.InitializeRequest{
				Config: map[string]interface{}{
					"connection_url":    connURL,
					"username_template": "foo-{{random 10}}-{{.DisplayName}}",
				},
				VerifyConnection: true,
			},
			expectedResp: dbplugin.InitializeResponse{
				Config: map[string]interface{}{
					"connection_url":    connURL,
					"username_template": "foo-{{random 10}}-{{.DisplayName}}",
				},
			},
			expectErr:         false,
			expectInitialized: true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			db := newClickhouse(DefaultUserNameTemplate)
			defer dbtesting.AssertClose(t, db)
			initResp, err := db.Initialize(context.Background(), test.initRequest)
			if test.expectErr && err == nil {
				t.Fatalf("err expected, got nil")
			}
			if !test.expectErr && err != nil {
				t.Fatalf("no error expected, got: %s", err)
			}
			require.Equal(t, test.expectedResp, initResp)
			require.Equal(t, test.expectInitialized, db.Initialized, "Initialized variable not set correctly")
		})
	}
}

func TestClickhouse_NewUser(t *testing.T) {
	displayName := "token"
	roleName := "testrole"

	type testCase struct {
		usernameTemplate string

		newUserReq dbplugin.NewUserRequest

		useSSL bool

		expectedUsernameRegex string
		expectErr             bool
	}

	tests := map[string]testCase{
		"name statements": {
			newUserReq: dbplugin.NewUserRequest{
				UsernameConfig: dbplugin.UsernameMetadata{
					DisplayName: displayName,
					RoleName:    roleName,
				},
				Statements: dbplugin.Statements{
					Commands: []string{
						`CREATE USER '{{name}}' IDENTIFIED BY '{{password}}';
						GRANT SELECT ON *.* TO '{{name}}';`,
					},
				},
				Password:   "09g8hanbdfkVSM",
				Expiration: time.Now().Add(time.Minute),
			},

			expectedUsernameRegex: `^v-token-testrole-[a-zA-Z0-9]{15}$`,
			expectErr:             false,
		},
		"name statements with SSL": {
			useSSL: true,
			newUserReq: dbplugin.NewUserRequest{
				UsernameConfig: dbplugin.UsernameMetadata{
					DisplayName: displayName,
					RoleName:    roleName,
				},
				Statements: dbplugin.Statements{
					Commands: []string{
						`CREATE USER '{{name}}' IDENTIFIED BY '{{password}}';
						GRANT SELECT ON *.* TO '{{name}}';`,
					},
				},
				Password:   "09g8hanbdfkVSM",
				Expiration: time.Now().Add(time.Minute),
			},

			expectedUsernameRegex: `^v-token-testrole-[a-zA-Z0-9]{15}$`,
			expectErr:             false,
		},
		"username statements": {
			newUserReq: dbplugin.NewUserRequest{
				UsernameConfig: dbplugin.UsernameMetadata{
					DisplayName: displayName,
					RoleName:    roleName,
				},
				Statements: dbplugin.Statements{
					Commands: []string{
						`CREATE USER '{{name}}' IDENTIFIED BY '{{password}}';
						GRANT SELECT ON *.* TO '{{name}}';`,
					},
				},
				Password:   "09g8hanbdfkVSM",
				Expiration: time.Now().Add(time.Minute),
			},

			expectedUsernameRegex: `^v-token-testrole-[a-zA-Z0-9]{15}$`,
			expectErr:             false,
		},
		"custom username template": {
			usernameTemplate: "foo-{{random 10}}-{{.RoleName | uppercase}}",

			newUserReq: dbplugin.NewUserRequest{
				UsernameConfig: dbplugin.UsernameMetadata{
					DisplayName: displayName,
					RoleName:    roleName,
				},
				Statements: dbplugin.Statements{
					Commands: []string{
						`CREATE USER '{{username}}' IDENTIFIED BY '{{password}}';
						GRANT SELECT ON *.* TO '{{username}}';`,
					},
				},
				Password:   "09g8hanbdfkVSM",
				Expiration: time.Now().Add(time.Minute),
			},

			expectedUsernameRegex: `^foo-[a-zA-Z0-9]{10}-TESTROLE$`,
			expectErr:             false,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {

			cleanup, connURL := clickhousehelper.PrepareTestContainer(t, test.useSSL, "admin_user", "secret")
			defer cleanup()

			connectionDetails := map[string]interface{}{
				"connection_url":    connURL,
				"username_template": test.usernameTemplate,
			}

			initReq := dbplugin.InitializeRequest{
				Config:           connectionDetails,
				VerifyConnection: true,
			}

			db := newClickhouse(DefaultUserNameTemplate)
			defer db.Close()
			_, err := db.Initialize(context.Background(), initReq)
			require.NoError(t, err)

			userResp, err := db.NewUser(context.Background(), test.newUserReq)
			if test.expectErr && err == nil {
				t.Fatalf("err expected, got nil")
			}
			if !test.expectErr && err != nil {
				t.Fatalf("no error expected, got: %s", err)
			}
			require.Regexp(t, test.expectedUsernameRegex, userResp.Username)

			connURLBuilder, err := NewConnStringBuilderFromConnString(connURL)
			if err != nil {
				t.Fatalf("unable to get a connection string builder from a connection string. err=%v", err.Error())
			}
			connURL, err = connURLBuilder.WithUsername(userResp.Username).WithPassword(test.newUserReq.Password).BuildConnectionString()
			if err != nil {
				t.Fatalf("unable to build connection string. err=%v", err.Error())
			}

			err = clickhousehelper.TestCredsExist(t, connURL)
			require.NoError(t, err, "Failed to connect with credentials")
		})
	}
}

func TestClickhouse_DeleteUser(t *testing.T) {
	displayName := "token"
	roleName := "testrole"

	newUserReq := dbplugin.NewUserRequest{
		UsernameConfig: dbplugin.UsernameMetadata{
			DisplayName: displayName,
			RoleName:    roleName,
		},
		Statements: dbplugin.Statements{
			Commands: []string{
				`CREATE USER '{{name}}' IDENTIFIED BY '{{password}}';
				 GRANT SELECT ON *.* TO '{{name}}';`,
			},
		},
		Password:   "09g8hanbdfkVSM",
		Expiration: time.Now().Add(time.Minute),
	}

	type testCase struct {
		usernameTemplate string

		newUserReq dbplugin.NewUserRequest
		delUserReq dbplugin.DeleteUserRequest

		useSSL bool

		expectedUsernameRegex string
		expectErr             bool
	}

	tests := map[string]testCase{
		"name statements": {
			newUserReq: newUserReq,
			delUserReq: dbplugin.DeleteUserRequest{
				Statements: dbplugin.Statements{
					Commands: []string{
						"DROP USER IF EXISTS '{{name}}'",
					},
				},
			},

			expectErr: false,
		},
		"username statements": {
			newUserReq: newUserReq,
			delUserReq: dbplugin.DeleteUserRequest{
				Statements: dbplugin.Statements{
					Commands: []string{
						"DROP USER IF EXISTS '{{username}}'",
					},
				},
			},
			expectErr: false,
		},
		"default delete statement": {
			newUserReq: newUserReq,
			delUserReq: dbplugin.DeleteUserRequest{},
			expectErr:  false,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {

			cleanup, connURL := clickhousehelper.PrepareTestContainer(t, test.useSSL, "admin_user", "secret")
			defer cleanup()

			connectionDetails := map[string]interface{}{
				"connection_url":    connURL,
				"username_template": test.usernameTemplate,
			}

			initReq := dbplugin.InitializeRequest{
				Config:           connectionDetails,
				VerifyConnection: true,
			}

			db := newClickhouse(DefaultUserNameTemplate)
			defer db.Close()
			_, err := db.Initialize(context.Background(), initReq)
			require.NoError(t, err)

			// Create User
			userResp, err := db.NewUser(context.Background(), test.newUserReq)
			if test.expectErr && err == nil {
				t.Fatalf("err expected, got nil")
			}
			if !test.expectErr && err != nil {
				t.Fatalf("no error expected, got: %s", err)
			}

			// Test user is present
			connURLBuilder, err := NewConnStringBuilderFromConnString(connURL)
			if err != nil {
				t.Fatalf("unable to get a connection string builder from a connection string. err=%v", err.Error())
			}
			connURL, err = connURLBuilder.WithUsername(userResp.Username).WithPassword(test.newUserReq.Password).BuildConnectionString()
			if err != nil {
				t.Fatalf("unable to build connection string. err=%v", err.Error())
			}

			err = clickhousehelper.TestCredsExist(t, connURL)
			require.NoError(t, err, "Failed to connect with credentials")

			// Update delete request
			test.delUserReq.Username = userResp.Username
			_, err = db.DeleteUser(context.Background(), test.delUserReq)
			if err != nil {
				t.Fatalf("no error expected. got: %s", err)
			}
			// Test connect should fail now
			err = clickhousehelper.TestCredsExist(t, connURL)
			require.Error(t, err, "user not removed. connection to clickhouse was a success")

		})
	}
}

func TestClickhouse_UpdateUser(t *testing.T) {
	displayName := "token"
	roleName := "testrole"

	newUserReq := dbplugin.NewUserRequest{
		UsernameConfig: dbplugin.UsernameMetadata{
			DisplayName: displayName,
			RoleName:    roleName,
		},
		Statements: dbplugin.Statements{
			Commands: []string{
				`CREATE USER '{{name}}' IDENTIFIED BY '{{password}}';
				 GRANT SELECT ON *.* TO '{{name}}';`,
			},
		},
		Password:   "09g8hanbdfkVSM",
		Expiration: time.Now().Add(time.Minute),
	}

	type testCase struct {
		usernameTemplate string

		newUserReq dbplugin.NewUserRequest
		updUserReq dbplugin.UpdateUserRequest

		useSSL bool

		expectedUsernameRegex string
		expectErr             bool
	}

	tests := map[string]testCase{
		"empty update user request": {
			newUserReq: newUserReq,
			updUserReq: dbplugin.UpdateUserRequest{
				Password:   nil,
				Expiration: nil,
			},
			expectErr: true,
		},
		"password update": {
			newUserReq: newUserReq,
			updUserReq: dbplugin.UpdateUserRequest{
				Password: &dbplugin.ChangePassword{
					NewPassword: "someNewPassword",
					Statements: dbplugin.Statements{
						Commands: []string{
							"ALTER USER IF EXISTS '{{name}}' IDENTIFIED BY '{{password}}';",
						},
					},
				},
				Expiration: nil,
			},
			expectErr: false,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {

			cleanup, connURL := clickhousehelper.PrepareTestContainer(t, test.useSSL, "admin_user", "secret")
			defer cleanup()

			connectionDetails := map[string]interface{}{
				"connection_url":    connURL,
				"username_template": test.usernameTemplate,
			}

			initReq := dbplugin.InitializeRequest{
				Config:           connectionDetails,
				VerifyConnection: true,
			}

			db := newClickhouse(DefaultUserNameTemplate)
			defer db.Close()
			_, err := db.Initialize(context.Background(), initReq)
			require.NoError(t, err)

			// Create User
			userResp, err := db.NewUser(context.Background(), test.newUserReq)
			if err != nil {
				t.Fatalf("err expected, got nil")
			}

			// Test user is present
			connURLBuilder, err := NewConnStringBuilderFromConnString(connURL)
			if err != nil {
				t.Fatalf("unable to get a connection string builder from a connection string. err=%v", err.Error())
			}
			connURL, err = connURLBuilder.WithUsername(userResp.Username).WithPassword(test.newUserReq.Password).BuildConnectionString()
			if err != nil {
				t.Fatalf("unable to build connection string. err=%v", err.Error())
			}

			err = clickhousehelper.TestCredsExist(t, connURL)
			require.NoError(t, err, "Failed to connect with credentials")

			// Update user request
			test.updUserReq.Username = userResp.Username
			_, err = db.UpdateUser(context.Background(), test.updUserReq)
			if test.expectErr && err == nil {
				t.Fatalf("err expected, got nil")
			}
			if !test.expectErr && err != nil {
				t.Fatalf("no error expected, got: %s", err)
			}
			if !test.expectErr {
				connURL, err = connURLBuilder.WithPassword(test.updUserReq.Password.NewPassword).BuildConnectionString()
				if err != nil {
					t.Fatalf("can't update connString with new updated password")
				}
				// Test connect should not fail
				err = clickhousehelper.TestCredsExist(t, connURL)
				require.NoError(t, err, "User Updated with success")
			}
		})
	}
}

func TestNew(t *testing.T) {
	type args struct {
		defaultUsernameTemplate string
		version                 string
	}
	tests := []struct {
		name              string
		args              args
		wantPlugInVersion string
		wantType          string
	}{
		{
			name: "Should return a New Clickhouse",
			args: args{
				defaultUsernameTemplate: "some_username_template",
				version:                 "0.0.1-test",
			},
			wantPlugInVersion: "0.0.1-test",
			wantType:          "clickhouse",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			gotFunc := New(tt.args.defaultUsernameTemplate, tt.args.version)
			gotInterface, err := gotFunc()
			if err != nil {
				t.Fatalf("error calling New(): error = %v", err.Error())
			}
			if got, ok := gotInterface.(dbplugin.DatabaseErrorSanitizerMiddleware); !ok {
				t.Errorf("New() result interface is not dbplugin.DatabaseErrorSanitizerMiddleware")
			} else {
				if got.PluginVersion().Version != tt.wantPlugInVersion {
					t.Errorf("New() Plugin version error. got=%s, want=%s", got.PluginVersion().Version, tt.wantPlugInVersion)
				}
				dbType, _ := got.Type()
				if dbType != tt.wantType {
					t.Errorf("New() DB Type error. got=%s, want=%s", dbType, tt.wantType)
				}
			}
		})
	}
}
