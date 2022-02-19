package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/blakewilliams/remote-development-manager/internal/clipboard"
	"github.com/blakewilliams/remote-development-manager/internal/config"
	"github.com/stretchr/testify/require"
)

func TestServer_Copy(t *testing.T) {
	path := UnixSocketPath() + ".test"
	client := http.Client{
		Timeout: time.Second * 10,
		Transport: &http.Transport{
			DialContext: func(_ctx context.Context, _network string, _address string) (net.Conn, error) {
				return net.Dial("unix", path)
			},
		},
	}

	nullLogger := log.New(io.Discard, "", log.LstdFlags)

	testClipboard := clipboard.NewTestClipboard()
	server := New(path, testClipboard, nullLogger, &config.RdmConfig{})

	listener, err := net.Listen("unix", server.path)
	defer os.Remove(server.path)
	require.NoError(t, err)

	go func() {
		err := server.Serve(context.Background(), listener)
		require.NoError(t, err)
	}()

	copyCommand := Command{
		Name:      "copy",
		Arguments: []string{"test 1 2 3"},
	}

	data, err := json.Marshal(copyCommand)
	require.NoError(t, err)

	_, err = client.Post("http://unix://"+path, "application/json", bytes.NewReader(data))
	require.NoError(t, err)

	require.Equal(t, "test 1 2 3", testClipboard.Buffer)

	pasteCommand := Command{
		Name:      "paste",
		Arguments: []string{},
	}

	data, err = json.Marshal(pasteCommand)
	require.NoError(t, err)

	result, err := client.Post("http://unix://"+path, "application/json", bytes.NewReader(data))
	require.NoError(t, err)

	body, err := io.ReadAll(result.Body)
	require.NoError(t, err)

	require.Equal(t, "test 1 2 3", string(body))
}

func TestServer_Run(t *testing.T) {
	path := UnixSocketPath() + ".test_run"
	client := http.Client{
		Timeout: time.Second * 10,
		Transport: &http.Transport{
			DialContext: func(_ctx context.Context, _network string, _address string) (net.Conn, error) {
				return net.Dial("unix", path)
			},
		},
	}

	tmpScript, err := ioutil.TempFile("", "tmpscript.sh")
	tmpScript.WriteString("#!/usr/bin/env bash\necho 'hi'")
	tmpScript.Chmod(0700)
	require.NoError(t, err)

	nullLogger := log.New(io.Discard, "", log.LstdFlags)

	server := New(
		path,
		clipboard.NewTestClipboard(),
		nullLogger,
		&config.RdmConfig{
			Commands: map[string]*config.UserCommand{
				"test": {ExecutablePath: tmpScript.Name(), LongRunning: false},
			},
		},
	)

	listener, err := net.Listen("unix", server.path)
	defer os.Remove(server.path)
	require.NoError(t, err)

	go func() {
		err := server.Serve(context.Background(), listener)
		require.NoError(t, err)
	}()

	runCommand := Command{
		Name:      "run",
		Arguments: []string{"test"},
	}

	data, err := json.Marshal(runCommand)
	require.NoError(t, err)

	response, err := client.Post("http://unix://"+path, "application/json", bytes.NewReader(data))
	require.NoError(t, err)

	content, err := io.ReadAll(response.Body)
	require.NoError(t, err)

	require.Equal(t, "hi\n", string(content))
}
