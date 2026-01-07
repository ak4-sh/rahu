package utils

import (
	"os"
)

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func ParseFile(path string) string {
	data, err := os.ReadFile(path)
	check(err)
	return string(data)
}
