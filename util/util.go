package util

import (
	"encoding/base64"
	"fmt"

	"github.com/satori/go.uuid"
)

func GenerateUUID() string {
	u1 := uuid.NewV4()
	return fmt.Sprintf("%s", u1)
}

func EncodeBase64(message string) (retour string) {
	base64Text := make([]byte, base64.StdEncoding.EncodedLen(len(message)))
	base64.StdEncoding.Encode(base64Text, []byte(message))
	return string(base64Text)
}

func DecodeBase64(message string) (retour string) {
	base64Text := make([]byte, base64.StdEncoding.DecodedLen(len(message)))
	base64.StdEncoding.Decode(base64Text, []byte(message))
	fmt.Printf("base64: %s\n", base64Text)
	return string(base64Text)
}
