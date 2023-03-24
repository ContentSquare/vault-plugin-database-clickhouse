package test

//
//func main() {
//	xxx := vault_plugin_database_clickhouse.New(vault_plugin_database_clickhouse.DefaultUserNameTemplate)
//	a, err := xxx()
//	if err != nil {
//		log.Errorf("error init connection. err=%v", err.Error())
//		return
//	}
//	log.Infof("%T", a)
//	c := a.(dbplugin.DatabaseErrorSanitizerMiddleware)
//	conf := map[string]interface{}{
//		"connection_url": fmt.Sprintf("clickhouse://%s:%d?username=%s&password=%s", "clickhouse-dev-shard1-r1-dev.eu-west-1.csq.io", 9000, "admin_user_account", "abcd1234"),
//		"hosts":          "clickhouse-dev-shard-r1.eu-west-1.csq.io",
//		"username":       "admin_user_account",
//		"password":       "abcd1234",
//	}
//	res, err := c.Initialize(context.TODO(), dbplugin.InitializeRequest{
//		Config:           conf,
//		VerifyConnection: true,
//	})
//	if err != nil {
//		log.Errorf("error init connection. err=%v", err.Error())
//		return
//	}
//
//	log.Infof("%+v", res)
//	res1, err := c.NewUser(context.TODO(), dbplugin.NewUserRequest{
//		UsernameConfig: dbplugin.UsernameMetadata{
//			DisplayName: "bob",
//			RoleName:    "role-bladibla",
//		},
//		Statements: dbplugin.Statements{
//			Commands: []string{
//				"CREATE USER '{{name}}' IDENTIFIED BY '{{password}}'",
//				"GRANT readonly TO '{{name}}'",
//			},
//		},
//		RollbackStatements: dbplugin.Statements{},
//		CredentialType:     0,
//		Password:           "bladibla",
//		PublicKey:          nil,
//		Expiration:         time.Time{},
//	})
//	if err != nil {
//		log.Errorf("error init connection. err=%v", err.Error())
//		return
//	}
//	log.Infof("%+v", res1)
//
//	//res1, err := c.DeleteUser(context.TODO(), dbplugin.DeleteUserRequest{
//	//	Username: "v-bob-role-bladi-Oxzj3GNWTe8zojd",
//	//	Statements: dbplugin.Statements{
//	//		Commands: []string{"DROP USER '{{username}}'"},
//	//	},
//	//})
//	if err != nil {
//		log.Errorf("error init connection. err=%v", err.Error())
//		return
//	}
//	log.Infof("%+v", res1)
//
//}
