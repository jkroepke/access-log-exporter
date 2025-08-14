package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const nginxConfig = `
user              nginx;
worker_processes  auto;

pid        /run/nginx.pid;

events {
    worker_connections  1024;
}

http {
	log_format accesslog_exporter '$http_host\t$request_method\t$status\t$request_time\t$request_length\t$bytes_sent';
	access_log syslog:server=host.docker.internal:8514,nohostname accesslog_exporter;

	server {
		listen       8080;
		server_name  localhost;

		location = /200 {
			return 200 "OK";
		}
		location = /204 {
			return 204 "No Content";
		}
		location = /404 {
			return 404 "Not Found";
		}
		location = /500 {
			return 500 "Internal Server Error";
		}

		location /proxy/ {
			proxy_pass http://127.0.0.1:8080/;
        }
	}
}
`

func TestIT(t *testing.T) {
	t.Parallel()

	termCh := make(chan os.Signal)
	returnCodeCh := make(chan ReturnCode)

	stdout := &bytes.Buffer{}

	wd, err := os.Getwd()
	require.NoError(t, err)

	moduleRoot, err := findModuleRoot(wd)
	require.NoError(t, err)

	go func() {
		returnCodeCh <- run(t.Context(), []string{
			"--config=" + moduleRoot + "/packaging/etc/access-log-exporter/config.yaml",
		}, stdout, termCh)
	}()

	time.Sleep(1 * time.Second)

	t.Cleanup(func() {
		termCh <- os.Interrupt
		require.Equal(t, ReturnCodeOK, <-returnCodeCh, stdout.String())
	})

	dockerImage := "nginx"
	if dockerImageEnv, ok := os.LookupEnv("DOCKER_IMAGE"); ok {
		dockerImage = dockerImageEnv
	}

	nginx, err := testcontainers.GenericContainer(t.Context(), testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image: dockerImage,
			ConfigModifier: func(config *container.Config) {
				config.Cmd = []string{
					"nginx-debug", "-g", "daemon off;",
				}
			},
			ExposedPorts: []string{
				"8080/tcp",
			},
			Env: map[string]string{
				"NGINX_ENTRYPOINT_QUIET_LOGS": "true",
			},
			Labels: map[string]string{
				"testcontainers": "true",
			},
			HostConfigModifier: func(hostConfig *container.HostConfig) {
				hostConfig.ExtraHosts = []string{"host.docker.internal:host-gateway"}
			},
			Files: []testcontainers.ContainerFile{
				{
					Reader:            strings.NewReader(nginxConfig),
					ContainerFilePath: "/etc/nginx/nginx.conf",
					FileMode:          0o644,
				},
			},
			WaitingFor: wait.ForListeningPort("8080/tcp").WithStartupTimeout(time.Second * 5),
		},
		Started: true,
	})

	testcontainers.CleanupContainer(t, nginx)

	containerLogs, _ := getContainerLogs(t, nginx)
	require.NoError(t, err, containerLogs)

	endpoint, err := nginx.PortEndpoint(t.Context(), "8080/tcp", "http")
	require.NoError(t, err, containerLogs)

	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete} {
		for _, code := range []string{"200", "204", "404", "500"} {
			req, err := http.NewRequestWithContext(t.Context(), method, endpoint+"/"+code, nil)
			require.NoError(t, err)

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)

			_, err = io.Copy(io.Discard, resp.Body)
			require.NoError(t, err)

			err = resp.Body.Close()
			require.NoError(t, err)

			req, err = http.NewRequestWithContext(t.Context(), method, endpoint+"/proxy/"+code, nil)
			require.NoError(t, err)

			resp, err = http.DefaultClient.Do(req)
			require.NoError(t, err)

			_, err = io.Copy(io.Discard, resp.Body)
			require.NoError(t, err)

			err = resp.Body.Close()
			require.NoError(t, err)
		}
	}

	time.Sleep(1 * time.Second)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://localhost:4040/metrics", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	err = resp.Body.Close()
	require.NoError(t, err)

	metrics := strings.TrimSpace(string(body))

	time.Sleep(1 * time.Second) // Wait for the exporter to process the logs

	require.Equal(t, 1332, strings.Count(metrics, "http_"))
}

func findModuleRoot(start string) (string, error) {
	dir := start
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("go.mod not found")
		}

		dir = parent
	}
}

func getContainerLogs(t *testing.T, ctr testcontainers.Container) (string, error) {
	t.Helper()

	cli, err := testcontainers.NewDockerClientWithOpts(t.Context())
	if err != nil {
		return "", fmt.Errorf("failed to create Docker client: %w", err)
	}

	logReader, err := cli.ContainerLogs(t.Context(), ctr.GetContainerID(), container.LogsOptions{
		ShowStderr: true,
		ShowStdout: true,
	})
	if err != nil {
		return "", fmt.Errorf("failed to get container logs: %w", err)
	}

	containerLogs, err := io.ReadAll(logReader)
	if err != nil {
		return "", fmt.Errorf("error reading container logs: %w", err)
	}

	return string(containerLogs), nil
}
