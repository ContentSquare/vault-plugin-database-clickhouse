package main

import (
	"log"
	"os"

	clickhouse "github.com/contentsquare/vault-plugin-database-clickhouse"
	"github.com/hashicorp/vault/sdk/database/dbplugin/v5"
)

// version is the plugin version reported to Vault (e.g. via `vault plugin
// list`). Vault validates it as strict semver, so the default must stay a
// plain numeric version. Override it at build time with:
//
//	-ldflags "-X main.version=X.Y.Z"
var (
	version = "0.0.0"
)

func main() {
	err := Run()
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
}

// Run instantiates a clickhouse object, and runs the RPC server for the plugin
func Run() error {
	f := clickhouse.New(clickhouse.DefaultUserNameTemplate, version)

	dbplugin.ServeMultiplex(f)

	return nil
}
