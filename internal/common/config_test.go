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
	"os"
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
			"password": "test",
			"viewId": "01234"
		},
		"token": "SEKRET",
		"logLevel": "info",
		"org": "test"
	}`)

var brokenConfig = []byte(`{ "foobar": { "baz": "biz" }`)

func TestMain(m *testing.M) {
	// Setup phase: Initialize the logger
	SetupLogger()

	// Run the tests
	code := m.Run()

	// Exit with the code from m.Run()
	os.Exit(code)
}

func TestReadConfig(t *testing.T) {
	expectedConfig := Config{
		ListenAddress: ":8000",
		Bluecat: &Bluecat{
			Account:  "test",
			BaseUrl:  "https://bluecat.example.com",
			Username: "test",
			Password: "test",
			ViewId:   "01234",
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
