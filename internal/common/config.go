package common

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
import (
	"dns-api-go/logger"
	"encoding/json"
	"io"

	"github.com/pkg/errors"
)

// Config is representation of the configuration data
type Config struct {
	ListenAddress string
	Token         string
	Bluecat       *Bluecat
	LogLevel      string
	Version       Version
	Org           string
	CIDRFile      string
}

type Bluecat struct {
	Account  string
	BaseUrl  string
	Username string
	Password string
	ViewId   string
}

// Version carries around the API version information
type Version struct {
	Version    string
	BuildStamp string
	GitHash    string
}

// ReadConfig decodes the configuration from an io Reader
func ReadConfig(r io.Reader) (Config, error) {
	var c Config
	logger.Info("decoding configuration...")
	if err := json.NewDecoder(r).Decode(&c); err != nil {
		return c, errors.Wrap(err, "unable to decode JSON message")
	}
	return c, nil
}
