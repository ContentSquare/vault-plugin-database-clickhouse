package vault_plugin_database_clickhouse

import (
	"reflect"
	"testing"

	_ "github.com/ClickHouse/clickhouse-go/v2"
)

func Test_connStringBuilder_buildConnectionString(t *testing.T) {
	type fields struct {
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
	tests := []struct {
		name    string
		fields  fields
		want    string
		wantErr bool
	}{
		{
			name: "Should build a simple connection string",
			fields: fields{
				host:     "someHost",
				port:     1234,
				username: "bob",
				password: "bladibla",
			},
			want:    "tcp://someHost:1234?password=bladibla&username=bob",
			wantErr: false,
		},
		{
			name: "Should build a simple connection string with a database",
			fields: fields{
				host:     "someHost",
				port:     1234,
				username: "bob",
				password: "bladibla",
				database: "someDatabase",
			},
			want:    "tcp://someHost:1234/someDatabase?password=bladibla&username=bob",
			wantErr: false,
		},
		{
			name: "Should build a simple connection string with a database and tls",
			fields: fields{
				host:     "someHost",
				port:     1234,
				username: "bob",
				password: "bladibla",
				database: "someDatabase",
				tls:      true,
			},
			want:    "tcp://someHost:1234/someDatabase?password=bladibla&secure=true&username=bob",
			wantErr: false,
		},
		{
			name: "Should build a simple connection string with a database and tls and skip_verify",
			fields: fields{
				host:          "someHost",
				port:          1234,
				username:      "bob",
				password:      "bladibla",
				database:      "someDatabase",
				tls:           true,
				tlsSkipVerify: true,
			},
			want:    "tcp://someHost:1234/someDatabase?password=bladibla&secure=true&skip_verify=true&username=bob",
			wantErr: false,
		},
		{
			name: "Should build a simple connection string with a database and debug",
			fields: fields{
				host:     "someHost",
				port:     1234,
				username: "bob",
				password: "bladibla",
				database: "someDatabase",
				debug:    true,
			},
			want:    "tcp://someHost:1234/someDatabase?debug=true&password=bladibla&username=bob",
			wantErr: false,
		},
		{
			name: "Should Add extra query params to the DSN",
			fields: fields{
				host:          "someHost",
				port:          1234,
				username:      "bob",
				password:      "bladibla",
				database:      "someDatabase",
				tls:           true,
				tlsSkipVerify: true,
				extra:         map[string]string{"someparam": "somevalue", "other_param": "bladibla"},
			},
			want:    "tcp://someHost:1234/someDatabase?other_param=bladibla&password=bladibla&secure=true&skip_verify=true&someparam=somevalue&username=bob",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &connStringBuilder{
				host:          tt.fields.host,
				port:          tt.fields.port,
				database:      tt.fields.database,
				debug:         tt.fields.debug,
				tls:           tt.fields.tls,
				tlsSkipVerify: tt.fields.tlsSkipVerify,
				username:      tt.fields.username,
				password:      tt.fields.password,
				extra:         tt.fields.extra,
			}
			got, err := c.BuildConnectionString()
			if (err != nil) != tt.wantErr {
				t.Errorf("BuildConnectionString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("BuildConnectionString() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_connStringBuilder_Check(t *testing.T) {
	type fields struct {
		host          string
		port          int
		database      string
		debug         bool
		tls           bool
		tlsSkipVerify bool
		username      string
		password      string
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{
			name: "Should return an error when host is empty",
			fields: fields{
				host:     "",
				port:     1234,
				username: "bob",
				password: "bladibla",
			},
			wantErr: true,
		},
		{
			name: "Should return an error when port is empty",
			fields: fields{
				host:     "someHost",
				username: "bob",
				password: "bladibla",
			},
			wantErr: true,
		},
		{
			name: "Should return an error when host and port is empty",
			fields: fields{
				username: "bob",
				password: "bladibla",
			},
			wantErr: true,
		},
		{
			name: "Should return no error when everything is ok",
			fields: fields{
				host: "someHost",
				port: 1234,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &connStringBuilder{
				host:          tt.fields.host,
				port:          tt.fields.port,
				database:      tt.fields.database,
				debug:         tt.fields.debug,
				tls:           tt.fields.tls,
				tlsSkipVerify: tt.fields.tlsSkipVerify,
				username:      tt.fields.username,
				password:      tt.fields.password,
			}
			if err := c.Check(); (err != nil) != tt.wantErr {
				t.Errorf("Check() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewConnStringBuilderFromConnString(t *testing.T) {
	type args struct {
		connString string
	}
	tests := []struct {
		name    string
		args    args
		want    *connStringBuilder
		wantErr bool
	}{
		{
			name: "Should return a Builder from a simple Connection String",
			args: args{
				connString: "tcp://someHost:1234/someDB?username={{username}}&password={{password}}&someOption=someValue",
			},
			want: &connStringBuilder{
				host:          "someHost",
				port:          1234,
				database:      "someDB",
				debug:         false,
				tls:           false,
				tlsSkipVerify: false,
				username:      "{{username}}",
				password:      "{{password}}",
				extra:         map[string]string{"someOption": "someValue"},
			},
			wantErr: false,
		},
		{
			name: "Should return a Builder from a more complex Connection String",
			args: args{
				connString: "tcp://someHost:1234/someDB?secure=true&skip_verify=true&debug=true&username={{username}}&password={{password}}&blah=dibla&someOption=someValue",
			},
			want: &connStringBuilder{
				host:          "someHost",
				port:          1234,
				database:      "someDB",
				debug:         true,
				tls:           true,
				tlsSkipVerify: true,
				username:      "{{username}}",
				password:      "{{password}}",
				extra:         map[string]string{"someOption": "someValue", "blah": "dibla"},
			},
			wantErr: false,
		},
		{
			name: "Should return an error on failed parsebool tls",
			args: args{
				connString: "tcp://someHost:1234/someDB?secure=bladibla",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Should return an error on failed parsebool skip_verify",
			args: args{
				connString: "tcp://someHost:1234/someDB?skip_verify=bladibla",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Should return an error on failed parsebool debug",
			args: args{
				connString: "tcp://someHost:1234/someDB?debug=bladibla",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Should return an error on failed parseint port",
			args: args{
				connString: "tcp://someHost:bladibla/someDB",
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewConnStringBuilderFromConnString(tt.args.connString)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewConnStringBuilderFromConnString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewConnStringBuilderFromConnString() got = \n%+v, want \n%+v", got, tt.want)
			}
		})
	}
}

func Test_connStringBuilder_WithHost(t *testing.T) {
	type fields struct {
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
	type args struct {
		host string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *connStringBuilder
	}{
		{
			name:   "Should add the host attribute",
			fields: fields{},
			args: args{
				host: "some.host.tld",
			},
			want: &connStringBuilder{
				host: "some.host.tld",
			},
		},
		{
			name: "Should replace the host attribute",
			fields: fields{
				host: "another.host.tld",
			},
			args: args{
				host: "some.host.tld",
			},
			want: &connStringBuilder{
				host: "some.host.tld",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &connStringBuilder{
				host:          tt.fields.host,
				port:          tt.fields.port,
				database:      tt.fields.database,
				debug:         tt.fields.debug,
				tls:           tt.fields.tls,
				tlsSkipVerify: tt.fields.tlsSkipVerify,
				username:      tt.fields.username,
				password:      tt.fields.password,
				extra:         tt.fields.extra,
			}
			if got := c.WithHost(tt.args.host); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("WithHost() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_connStringBuilder_WithPort(t *testing.T) {
	type fields struct {
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
	type args struct {
		port int
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *connStringBuilder
	}{
		{
			name:   "Should add the port attribute",
			fields: fields{},
			args: args{
				port: 1234,
			},
			want: &connStringBuilder{
				port: 1234,
			},
		},
		{
			name:   "Should replace the port attribute",
			fields: fields{port: 9876},
			args: args{
				port: 1234,
			},
			want: &connStringBuilder{
				port: 1234,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &connStringBuilder{
				host:          tt.fields.host,
				port:          tt.fields.port,
				database:      tt.fields.database,
				debug:         tt.fields.debug,
				tls:           tt.fields.tls,
				tlsSkipVerify: tt.fields.tlsSkipVerify,
				username:      tt.fields.username,
				password:      tt.fields.password,
				extra:         tt.fields.extra,
			}
			if got := c.WithPort(tt.args.port); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("WithPort() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_connStringBuilder_WithDatabase(t *testing.T) {
	type fields struct {
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
	type args struct {
		database string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *connStringBuilder
	}{
		{
			name:   "Should add the database attribute",
			fields: fields{},
			args:   args{database: "somedatabase"},
			want: &connStringBuilder{
				database: "somedatabase",
			},
		},
		{
			name: "Should replace the database attribute",
			fields: fields{
				database: "otherdatabase",
			},
			args: args{database: "somedatabase"},
			want: &connStringBuilder{database: "somedatabase"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &connStringBuilder{
				host:          tt.fields.host,
				port:          tt.fields.port,
				database:      tt.fields.database,
				debug:         tt.fields.debug,
				tls:           tt.fields.tls,
				tlsSkipVerify: tt.fields.tlsSkipVerify,
				username:      tt.fields.username,
				password:      tt.fields.password,
				extra:         tt.fields.extra,
			}
			if got := c.WithDatabase(tt.args.database); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("WithDatabase() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_connStringBuilder_WithTLS(t *testing.T) {
	type fields struct {
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
	type args struct {
		skipVerify bool
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *connStringBuilder
	}{
		{
			name:   "Should add the TLS Attribute with Verify",
			fields: fields{},
			args: args{
				skipVerify: false,
			},
			want: &connStringBuilder{
				tls: true,
			},
		},
		{
			name:   "Should add the TLS Attribute without Verify",
			fields: fields{},
			args: args{
				skipVerify: true,
			},
			want: &connStringBuilder{
				tls:           true,
				tlsSkipVerify: true,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &connStringBuilder{
				host:          tt.fields.host,
				port:          tt.fields.port,
				database:      tt.fields.database,
				debug:         tt.fields.debug,
				tls:           tt.fields.tls,
				tlsSkipVerify: tt.fields.tlsSkipVerify,
				username:      tt.fields.username,
				password:      tt.fields.password,
				extra:         tt.fields.extra,
			}
			if got := c.WithTLS(tt.args.skipVerify); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("WithTLS() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_connStringBuilder_WithDebug(t *testing.T) {
	type fields struct {
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
	tests := []struct {
		name   string
		fields fields
		want   *connStringBuilder
	}{
		{
			name:   "Should add the debug attribute",
			fields: fields{},
			want: &connStringBuilder{
				debug: true,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &connStringBuilder{
				host:          tt.fields.host,
				port:          tt.fields.port,
				database:      tt.fields.database,
				debug:         tt.fields.debug,
				tls:           tt.fields.tls,
				tlsSkipVerify: tt.fields.tlsSkipVerify,
				username:      tt.fields.username,
				password:      tt.fields.password,
				extra:         tt.fields.extra,
			}
			if got := c.WithDebug(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("WithDebug() = %v, want %v", got, tt.want)
			}
		})
	}
}
