package utils

import (
	"fmt"
	"testing"
)

func TestHashPassword(t *testing.T) {
	password := "ambre7.4"
	hashpassword, err := HashPassword(password)
	if err == nil {
		fmt.Println(hashpassword)
	}
}
