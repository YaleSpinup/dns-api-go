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
	"context"
	"encoding/base64"
	"os"

	"dns-api-go/internal/api"
	"dns-api-go/internal/common"
	"dns-api-go/logger"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/awslabs/aws-lambda-go-api-proxy/gorillamux"
	"go.uber.org/zap"
)

var muxAdapter *gorillamux.GorillaMuxAdapterV2

func init() {
	logger.InitializeDefault()

	configEnv := os.Getenv("API_CONFIG")
	if configEnv == "" {
		logger.Fatal("API_CONFIG environment variable is required")
	}

	c, err := base64.StdEncoding.DecodeString(configEnv)
	if err != nil {
		logger.Info("API_CONFIG is not base64 encoded")
		c = []byte(configEnv)
	}

	config, err := common.ReadConfig(bytes.NewReader(c))
	if err != nil {
		logger.Fatal("Unable to read configuration", zap.Error(err))
	}

	// set APP_ENV variable based on org
	var appEnv string
	switch config.Org {
	case "dev":
		appEnv = "development"
	case "sstst":
		appEnv = "test"
	case "ss":
		appEnv = "production"
	default:
		logger.Error("Invalid org in config. Defaulting to development")
		appEnv = "development"
	}
	os.Setenv("APP_ENV", appEnv)

	// Sync the default logger before switching to a new configuration
	logger.Sync()
	logger.SetLogLevel(appEnv, config.LogLevel)

	app, err := api.NewApp(config)
	if err != nil {
		logger.Fatal("Failed to initialize application", zap.Error(err))
	}

	muxAdapter = gorillamux.NewV2(app.Router())
	logger.Info("Lambda handler initialized successfully")
}

func Handler(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	return muxAdapter.ProxyWithContext(ctx, req)
}

func main() {
	lambda.Start(Handler)
}

