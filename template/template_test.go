package template

import (
	"github.com/stretchr/testify/require"
	"log"
	"os"
	"path/filepath"
	"strings"
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

func TestExpandIpxe(t *testing.T) {
	macros := map[string]string{
		"netboot_name":                 "netboot",
		"netboot_domain":               "example.org",
		"netboot_version":              "1.1.1",
		"netboot_ntp_server":           "ntpserver.example.org",
		"netboot_timeout_microseconds": "3000",
	}
	templateData := AutoexecTemplate
	//log.Printf("before: %s\n", string(templateData))
	expandedData, err := ExpandTemplate(templateData, macros)
	require.Nil(t, err)
	//log.Printf("after: %s\n", string(expandedData))

	expectedData, err := os.ReadFile(filepath.Join("testdata", "expanded.ipxe"))
	require.Nil(t, err)
	expandedLines := strings.Split(string(expandedData), "\n")
	expandedCount := len(expandedLines)
	expectedLines := strings.Split(string(expectedData), "\n")
	expectedCount := len(expectedLines)
	require.Equal(t, expectedCount, expandedCount)
	for i, expectedLine := range expectedLines {
		require.Equal(t, expectedLine, expandedLines[i])
	}
	require.Equal(t, expectedData, expandedData)
}

func TestExpandMissingValue(t *testing.T) {
	macros := map[string]string{}
	templateData := AutoexecTemplate
	_, err := ExpandTemplate(templateData, macros)
	require.NotNil(t, err)
	require.ErrorContains(t, err, "unexpanded template macro:")
}
