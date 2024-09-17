// Copyright (c) OpenFaaS Author(s) 2019. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

package types

import (
	"github.com/openfaas/go-sdk"
	"os"

	"github.com/openfaas/faas-provider/auth"
)

func GetCredentials() *sdk.BasicAuth {
	var credentials *sdk.BasicAuth

	if val, ok := os.LookupEnv("basic_auth"); ok && len(val) > 0 {
		if val == "true" || val == "1" {

			reader := auth.ReadBasicAuthFromDisk{}

			if val, ok := os.LookupEnv("secret_mount_path"); ok && len(val) > 0 {
				reader.SecretMountPath = os.Getenv("secret_mount_path")
			}

			res, err := reader.Read()
			if err != nil {
				panic(err)
			}

			credentials.Username = res.User
			credentials.Password = res.Password
		}
	}
	return credentials
}
