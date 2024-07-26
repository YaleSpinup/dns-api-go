package common

import "dns-api-go/logger"

func SetupLogger() {
	logger.InitializeDefault()
	defer logger.Sync()
}
