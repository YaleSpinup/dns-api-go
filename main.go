/*
Copyright © 2023 Yale University

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <http://www.gnu.org/licenses/>.
*/
package main

import (
	"bytes"
	"encoding/base64"
	"flag"
	"fmt"
	"go.uber.org/zap"
	"io"
	"net/http"
	"os"

	"dns-api-go/internal/api"
	"dns-api-go/internal/common"

	"dns-api-go/logger"
)

var (
	// Version is the main version number
	Version = "0.0.0"

	// Buildstamp is the timestamp the binary was built, it should be set at buildtime with ldflags
	Buildstamp = "No BuildStamp Provided"

	// Githash is the git sha of the built binary, it should be set at buildtime with ldflags
	Githash = "No Git Commit Provided"

	configFileName = flag.String("config", "config/config.json", "Configuration file.")
	version        = flag.Bool("version", false, "Display version information and exit.")
)

func main() {
	flag.Parse()
	if *version {
		vers()
	}

	logger.InitializeDefault()

	cwd, err := os.Getwd()
	if err != nil {
		logger.Fatal("unable to get working directory")
	}
	logger.Info("Starting dns-api-go", zap.String("version", Version), zap.String("cwd", cwd))

	config, err := common.ReadConfig(configReader())
	if err != nil {
		logger.Fatal("Unable to read configuration from:", zap.Error(err))
	}

	// set APP_ENV variable based on org in config.go
	var appEnv string
	if config.Org == "dev" {
		appEnv = "development"
	} else if config.Org == "sstst" {
		appEnv = "test"
	} else if config.Org == "ss" {
		appEnv = "production"
	} else {
		// Default to development if org is not set
		logger.Error("Invalid org in config.go. Defaulting to development")
		appEnv = "development"
	}

	os.Setenv("APP_ENV", appEnv)

	config.Version = common.Version{
		Version:    Version,
		BuildStamp: Buildstamp,
		GitHash:    Githash,
	}

	// Sync the default logger before switching to a new configuration
	logger.Sync()
	// Initialize new logger based on environment and log level
	logger.SetLogLevel(appEnv, config.LogLevel)
	defer logger.Sync()

	if config.LogLevel == "debug" {
		logger.Debug("Starting profiler on 127.0.0.1:6080")
		go http.ListenAndServe("127.0.0.1:6080", nil)
	}
	logger.Debug("loaded configuration", zap.Any("config", config))

	if err := api.NewServer(config); err != nil {
		logger.Fatal("Server initialization failed", zap.Error(err))
	}
}

func configReader() io.Reader {
	if configEnv := os.Getenv("API_CONFIG"); configEnv != "" {
		logger.Info("reading configuration from API_CONFIG environment")

		c, err := base64.StdEncoding.DecodeString(configEnv)
		if err != nil {
			logger.Info("API_CONFIG is not base64 encoded")
			c = []byte(configEnv)
		}

		return bytes.NewReader(c)
	}

	logger.Info("reading configuration", zap.String("file", *configFileName))

	configFile, err := os.Open(*configFileName)
	if err != nil {
		logger.Fatal("unable to open config file", zap.Error(err))
	}

	c, err := io.ReadAll(configFile)
	if err != nil {
		logger.Fatal("unable to read config file", zap.Error(err))
	}

	return bytes.NewReader(c)
}

func vers() {
	fmt.Printf("dns-api-go Version: %s\n", Version)
	os.Exit(0)
}
