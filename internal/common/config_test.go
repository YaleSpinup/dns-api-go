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
package common

import (
	"bytes"
	"reflect"
	"testing"
)

var testConfig = []byte(
	`{
		"listenAddress": ":8000",
		"bluecat": {
			"account": "test",
			"baseUrl": "https://bluecat.example.com",
			"username": "test",
			"password": "test"
		},
		"account": {
			"region": "us-east-1",
			"akid": "key1",
			"secret": "secret1",
			"role": "uber-role",
			"externalId": "foobar"
		},
		"token": "SEKRET",
		"logLevel": "info",
		"org": "test"
	}`)

var brokenConfig = []byte(`{ "foobar": { "baz": "biz" }`)

func TestReadConfig(t *testing.T) {
	expectedConfig := Config{
		ListenAddress: ":8000",
		Bluecat: &Bluecat{
			Account:  "test",
			BaseUrl:  "https://bluecat.example.com",
			Username: "test",
			Password: "test",
		},
		Account: Account{
			Region:     "us-east-1",
			Akid:       "key1",
			Secret:     "secret1",
			Role:       "uber-role",
			ExternalID: "foobar",
		},
		Token:    "SEKRET",
		LogLevel: "info",
		Org:      "test",
	}

	actualConfig, err := ReadConfig(bytes.NewReader(testConfig))
	if err != nil {
		t.Error("Failed to read config", err)
	}

	if !reflect.DeepEqual(actualConfig, expectedConfig) {
		t.Errorf("Expected config to be %+v\n got %+v", expectedConfig, actualConfig)
	}

	_, err = ReadConfig(bytes.NewReader(brokenConfig))
	if err == nil {
		t.Error("expected error reading config, got nil")
	}
}
