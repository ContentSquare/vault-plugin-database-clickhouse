package main

import (
	"log"
	"os"

	clickhouse "github.com/contentsquare/vault-plugin-database-clickhouse"
	"github.com/hashicorp/vault/sdk/database/dbplugin/v5"
)

var (
	version string
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
