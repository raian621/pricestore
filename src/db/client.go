package db

import (
	"fmt"
)

func CreateDbConnString(
	user, password, host string, port uint, name string,
) string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s", user, password, host, port,
		name)
}
