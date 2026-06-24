package services

import (
	"dns-api-go/internal/common"
	"os"
	"testing"
)

// TestMain initializes the package-level logger. RecordService and
// IpAddressService both call logger.Info/Error directly; without this the
// nil package-level zap.Logger causes test binaries to wedge inside
// zap.(*Logger).check rather than failing cleanly.
func TestMain(m *testing.M) {
	common.SetupLogger()
	os.Exit(m.Run())
}
