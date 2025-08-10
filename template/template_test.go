package template

import (
	"github.com/stretchr/testify/require"
	"log"
	"testing"
)

func TestExpandTemplate(t *testing.T) {
	data := "test line with {{macroname}} macro\n"
	log.Printf("before: %s\n", data)
	macros := make(map[string]string)
	macros["macroname"] = "expanded_value"
	expanded, err := ExpandTemplate([]byte(data), macros)
	require.Nil(t, err)
	require.Equal(t, "test line with expanded_value macro\n", string(expanded))
	log.Printf("after: %s\n", expanded)
}
