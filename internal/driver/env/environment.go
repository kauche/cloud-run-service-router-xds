package env

type Environments struct {
	Port int `envconfig:"PORT" required:"true"`
}
