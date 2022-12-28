package envconfig

import (
	"github.com/kelseyhightower/envconfig"

	"github.com/kauche/cloud-run-service-router-xds/internal/driver/env"
)

func GetEnvironments() (*env.Environments, error) {
	var e env.Environments
	if err := envconfig.Process("", &e); err != nil {
		return nil, err
	}

	return &e, nil
}
