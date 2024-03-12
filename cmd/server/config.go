package main

import (
	"fmt"
	"math"
	"os"
	"time"
)

// httpConfig is the configuration for the HTTP server.
type httpConfig struct {
	addr            string
	readTimeout     time.Duration
	writeTimeout    time.Duration
	idleTimeout     time.Duration
	shutdownTimeout time.Duration
}

// config is the configuration for the server command.
type config struct {
	http httpConfig
}

// defaultConfig returns a config with sane default values.
func defaultConfig() config {
	return config{
		http: httpConfig{
			addr:            ":8888",
			readTimeout:     time.Second * 5,
			writeTimeout:    time.Second * 10,
			idleTimeout:     time.Second * 120,
			shutdownTimeout: time.Second * 15,
		},
	}
}

// envMap maps environment variable names to fields in the config struct.
var envMap = map[string]func(v string, c *config) error{
	"HTTP_ADDR": func(v string, c *config) error {
		c.http.addr = v
		return nil
	},
	"HTTP_READ_TIMEOUT": func(v string, c *config) error {
		return confDuration(v, &c.http.readTimeout, 0, math.MaxInt64)
	},
	"HTTP_WRITE_TIMEOUT": func(v string, c *config) error {
		return confDuration(v, &c.http.writeTimeout, 0, math.MaxInt64)
	},
	"HTTP_IDLE_TIMEOUT": func(v string, c *config) error {
		return confDuration(v, &c.http.idleTimeout, 0, math.MaxInt64)
	},
	"HTTP_SHUTDOWN_TIMEOUT": func(v string, c *config) error {
		return confDuration(v, &c.http.shutdownTimeout, 0, math.MaxInt64)
	},
}

// configFromEnv returns a config with values from the environment. It falls
// back to default values for any missing environment variables.
//
// It does a best effort to validate provided values, so that mistakes are
// caught ASAP. However, there is no guarantee that the returned config
// is valid and will work.
func configFromEnv() (config, error) {
	c := defaultConfig()

	for key, mf := range envMap {
		if val, ok := os.LookupEnv(key); ok {
			if err := mf(val, &c); err != nil {
				return c, fmt.Errorf("invalid env variable %s: %w", key, err)
			}
		}
	}

	return c, nil
}

// confDuration attempts to parse v into tgt and checks if the result is in
// the provided range (inclusive).
func confDuration(v string, tgt *time.Duration, min, max time.Duration) error {
	dur, err := time.ParseDuration(v)
	if err != nil {
		return err
	}

	if dur < min || dur > max {
		return fmt.Errorf("duration %s not in range [%s, %s] (inclusive)", dur, min, max)
	}

	*tgt = dur

	return nil
}
