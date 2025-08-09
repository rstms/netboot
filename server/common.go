package server

import (
	common "github.com/rstms/go-common"
)

func FormatJSON(v any) string {
	return common.FormatJSON(v)
}

func IsDir(path string) bool {
	return common.IsDir(path)
}

func IsFile(path string) bool {
	return common.IsFile(path)
}

func Fatal(err error) error {
	return common.Fatal(err)
}

func Fatalf(format string, args ...interface{}) error {
	return common.Fatalf(format, args...)
}
