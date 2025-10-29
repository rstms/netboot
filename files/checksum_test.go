package files

import (
	"github.com/stretchr/testify/require"
	"log"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSha512(t *testing.T) {
	file := filepath.Join("testdata", "zipme")
	sum, err := CalculateSHA512(file)
	require.Nil(t, err)
	log.Printf("sum=%s\n", sum)
	out, err := exec.Command("sha512", file).Output()
	require.Nil(t, err)
	fields := strings.Fields(string(out))
	control := fields[len(fields)-1]
	log.Printf("ctl=%s\n", control)
	require.Equal(t, sum, control)
}
