package util

import (
	"fmt"

	"github.com/satori/go.uuid"
)

func GenerateUUID() string {
	u1 := uuid.NewV4()
	return fmt.Sprintf("%s", u1)
}
