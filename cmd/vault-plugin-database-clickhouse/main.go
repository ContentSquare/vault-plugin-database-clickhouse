package main

import (
	"log"
	"os"

	"github.com/hashicorp/vault/sdk/database/dbplugin/v5"
	clickhouse "github.com/vfoucault/vault-plugin-database-clickhouse"
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
	var f func() (interface{}, error)
	f = clickhouse.New(clickhouse.DefaultUserNameTemplate, version)

	dbplugin.ServeMultiplex(f)

	return nil
}
