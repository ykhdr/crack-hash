package mongo

type ClientConfig struct {
	URI      string `kdl:"uri"`
	Username string `kdl:"username"`
	Password string `kdl:"password"`
}

type Config struct {
	ClientConfig
	Database string `kdl:"database"`
}
