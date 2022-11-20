package flag

import (
	"errors"
	"flag"
	"fmt"
	"time"

	internal_flag "github.com/kauche/cloud-run-service-router-xds/internal/driver/flag"
)

func GetFlags() (*internal_flag.Flags, error) {
	port := flag.Int("port", 5000, "Port number to listen")
	project := flag.String("project", "", "Google Cloud Project ID")
	location := flag.String("location", "", "Google Cloud Run Location")
	period := flag.String("sync-period", "", "Period to sync Services from Google Cloud Run")

	flag.Parse()

	if *project == "" {
		return nil, errors.New("project is empty")
	}

	if *location == "" {
		return nil, errors.New("location is empty")
	}

	if *period == "" {
		return nil, errors.New("sync-period is empty")
	}

	duration, err := time.ParseDuration(*period)
	if err != nil {
		return nil, fmt.Errorf("duration cannot be parsed: %w", err)
	}

	return &internal_flag.Flags{
		Port:       *port,
		Project:    *project,
		Location:   *location,
		SyncPeriod: duration,
	}, nil
}
