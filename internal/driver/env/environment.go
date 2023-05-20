package env

type Environments struct {
	Port                 int    `envconfig:"PORT" required:"true"`
	CloudRunEmulatorHost string `envconfig:"CLOUD_RUN_EMULATOR_HOST"`
}
