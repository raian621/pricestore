package db

import (
	"fmt"

	secret "github.com/andrewbenton/go-secrets"
)

func CreateDbConnString(
	user string, password secret.Secret[string], host string, port uint,
	name string,
) secret.Secret[string] {
	return secret.Make(fmt.Sprintf("postgres://%s:%s@%s:%d/%s", user, password,
		host, port, name))
}
