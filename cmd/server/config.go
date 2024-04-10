package main

import (
	"errors"
	"fmt"
	"math"
	"os"
	"strconv"
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

type dbConfig struct {
	file    string
	migrate bool
}

// config is the configuration for the server command.
type config struct {
	http httpConfig
	db   dbConfig
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
		db: dbConfig{
			file:    "househunt.db",
			migrate: true,
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
	"DB_FILENAME": func(v string, c *config) error {
		return confString(v, &c.db.file, 1, math.MaxInt64)
	},
	"DB_MIGRATE": func(v string, c *config) error {
		return confBool(v, &c.db.migrate)
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

	var errSum error
	for key, mf := range envMap {
		if val, ok := os.LookupEnv(key); ok {
			if err := mf(val, &c); err != nil {
				errSum = errors.Join(errSum, fmt.Errorf("invalid env variable %s: %w", key, err))
			}
		}
	}

	return c, errSum
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

func confString(v string, tgt *string, minLen, maxLen int) error {
	if len(v) < minLen || len(v) > maxLen {
		return fmt.Errorf("string length %d not in range [%d, %d] (inclusive)", len(v), minLen, maxLen)
	}

	*tgt = v

	return nil
}

func confBool(v string, tgt *bool) error {
	b, err := strconv.ParseBool(v)
	if err != nil {
		return err
	}

	*tgt = b

	return nil
}
