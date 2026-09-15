package message

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestMessageInit(t *testing.T) {
	require.NotNil(t, NetbootConfig{})
	require.NotNil(t, HostState{})
	require.NotNil(t, NetbootResponse{})
	require.NotNil(t, NetbootAddHostResponse{})
	require.NotNil(t, NetbootListHostsResponse{})
	require.NotNil(t, NetbootHostStatusResponse{})
	require.NotNil(t, NetbootDeleteHostResponse{})
	require.NotNil(t, NetbootDistFilesResponse{})
}
