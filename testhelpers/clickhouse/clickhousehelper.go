package clickhousehelper

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"testing"

	"github.com/hashicorp/vault/helper/testhelpers/docker"
)

type Config struct {
	docker.ServiceHostPort
	ConnString string
}

var _ docker.ServiceConfig = &Config{}

func PrepareTestContainer(t *testing.T, useTLS bool, adminUser, adminPassword string) (func(), string) {
	if os.Getenv("CLICKHOUSE_URL") != "" {
		return func() {}, os.Getenv("CLICKHOUSE_URL")
	}

	imageVersion := "22-alpine"
	extraCopy := map[string]string{}
	ports := []string{"9000/tcp"}
	if useTLS {
		if err := cenCACertificates("testhelpers/resources/certs"); err != nil {
			t.Fatalf("unable to generate SSL Certificates. err=%v", err.Error())
		}
		extraCopy["testhelpers/resources/certs"] = "/etc/clickhouse-server/certs"
		extraCopy["testhelpers/resources/config.xml"] = "/etc/clickhouse-server/config.xml"
		ports = []string{"9440/tcp"}
	}
	runner, err := docker.NewServiceRunner(docker.RunOptions{
		ImageRepo:     "clickhouse/clickhouse-server",
		ImageTag:      imageVersion,
		ContainerName: "clickhouse-server",
		Env: []string{
			fmt.Sprintf("CLICKHOUSE_USER=%s", adminUser),
			fmt.Sprintf("CLICKHOUSE_PASSWORD=%s", adminPassword),
			"CLICKHOUSE_DEFAULT_ACCESS_MANAGEMENT=1",
		},
		CopyFromTo:      extraCopy,
		Ports:           ports,
		DoNotAutoRemove: false,
	})
	if err != nil {
		t.Fatalf("could not start docker clickhouse: %s", err)
	}

	svc, err := runner.StartService(context.Background(), func(ctx context.Context, host string, port int) (docker.ServiceConfig, error) {
		hostIP := docker.NewServiceHostPort(host, port)
		q := make(url.Values)
		q.Set("username", adminUser)
		q.Set("password", adminPassword)
		if useTLS {
			q.Set("secure", "true")
			q.Set("skip_verify", "true")
		}
		dsn := (&url.URL{
			Scheme:   "tcp",
			Host:     hostIP.Address(),
			RawQuery: q.Encode(),
		}).String()

		db, err := sql.Open("clickhouse", dsn)
		if err != nil {
			return nil, err
		}
		defer db.Close()
		err = db.Ping()
		if err != nil {
			return nil, err
		}

		return &Config{ServiceHostPort: *hostIP, ConnString: dsn}, nil
	})
	if err != nil {
		t.Fatalf("could not start docker clickhouse: %s", err)
	}

	return svc.Cleanup, svc.Config.(*Config).ConnString
}

func TestCredsExist(t testing.TB, connURL string) error {

	db, err := sql.Open("clickhouse", connURL)
	if err != nil {
		return err
	}
	defer db.Close()
	return db.Ping()
}
