package server

import (
	"github.com/stretchr/testify/require"
	"log"
	"os"
	"path/filepath"
	"testing"
)

func TestNetbootServerPartitionTemplateColons(t *testing.T) {
	templateFile := filepath.Join("testdata", "partition_template.colons")
	templateBytes, err := os.ReadFile(templateFile)
	require.Nil(t, err)
	formattedBytes, err := formatPartitionTemplate(templateBytes)
	require.Nil(t, err)
	log.Printf("BEGIN\n%sEND\n", string(formattedBytes))
}

func TestNetbootServerPartitionTemplateLines(t *testing.T) {
	templateFile := filepath.Join("testdata", "partition_template.lines")
	templateBytes, err := os.ReadFile(templateFile)
	require.Nil(t, err)
	formattedBytes, err := formatPartitionTemplate(templateBytes)
	require.Nil(t, err)
	log.Printf("BEGIN\n%sEND\n", string(formattedBytes))
}
