package utils

import (
	"fmt"
	"testing"
)

func TestIDGen(t *testing.T) {
	pre := "w_"
	keys := []string{
		"おいしい",
		"形容词",
		"1",
		"美味的",
	}
	HashID := GenerateID(pre, keys...)
	fmt.Println(HashID)
}
