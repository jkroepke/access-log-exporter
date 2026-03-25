package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
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
	log_format access_log_exporter '$http_host\t$request_method\t$status\t$request_completion\t$request_time\t$request_length\t$bytes_sent';
	access_log syslog:server=host.docker.internal:8514,nohostname access_log_exporter;

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

		location = /stub_status {
			stub_status;
		}
	}
}
`

func TestIT(t *testing.T) {
	t.Parallel()

	termCh := make(chan os.Signal)
	returnCodeCh := make(chan ReturnCode, 1)

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

	stdout := &bytes.Buffer{}

	wd, err := os.Getwd()
	require.NoError(t, err)

	moduleRoot, err := findModuleRoot(wd)
	require.NoError(t, err)

	go func() {
		returnCodeCh <- run(t.Context(), []string{
			"--config=" + moduleRoot + "/packaging/etc/access-log-exporter/config.yaml",
			"--nginx.scrape-url=" + endpoint + "/stub_status",
			"--web.listen-address=127.0.0.1:54321",
			"--debug.enable=true",
		}, stdout, termCh)
	}()

	time.Sleep(1 * time.Second)

	t.Cleanup(func() {
		require.Equal(t, ReturnCodeOK, <-returnCodeCh, stdout.String())
	})

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

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://127.0.0.1:54321/metrics", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	err = resp.Body.Close()
	require.NoError(t, err)

	metrics := strings.TrimSpace(string(body))

	time.Sleep(1 * time.Second) // Wait for the exporter to process the logs

	require.Equal(t, 1, strings.Count(metrics, "log_parse_errors_total 0"), metrics)
	require.Equal(t, 448, strings.Count(metrics, "http_request_duration_seconds_"), metrics)
	require.Equal(t, 322, strings.Count(metrics, "http_request_size_bytes"), metrics)
	require.Equal(t, 34, strings.Count(metrics, "http_requests_completed_total"), metrics)
	require.Equal(t, 34, strings.Count(metrics, "http_requests_total"), metrics)
	require.Equal(t, 320, strings.Count(metrics, "http_response_size_bytes_"), metrics)
	require.Equal(t, 21, strings.Count(metrics, "nginx_"), metrics)

	termCh <- syscall.SIGTERM
}

func TestIT_HTTPS(t *testing.T) {
	t.Parallel()

	termCh := make(chan os.Signal)
	returnCodeCh := make(chan ReturnCode, 1)

	// Generate self-signed TLS certs
	certFile, keyFile := generateTestCerts(t)

	stdout := &bytes.Buffer{}

	wd, err := os.Getwd()
	require.NoError(t, err)

	moduleRoot, err := findModuleRoot(wd)
	require.NoError(t, err)

	go func() {
		returnCodeCh <- run(t.Context(), []string{
			"--config=" + moduleRoot + "/packaging/etc/access-log-exporter/config.yaml",
			"--web.listen-address=127.0.0.1:54322",
			"--web.tls-cert-file=" + certFile,
			"--web.tls-key-file=" + keyFile,
		}, stdout, termCh)
	}()

	time.Sleep(1 * time.Second)

	t.Cleanup(func() {
		require.Equal(t, ReturnCodeOK, <-returnCodeCh, stdout.String())
	})

	// Create HTTPS client that skips cert verification (self-signed)
	tlsClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, //nolint:gosec // test uses self-signed cert
			},
		},
	}

	// Test /health endpoint over HTTPS
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://127.0.0.1:54322/health", nil)
	require.NoError(t, err)

	resp, err := tlsClient.Do(req)
	require.NoError(t, err, "HTTPS request to /health failed")
	require.Equal(t, http.StatusOK, resp.StatusCode)

	err = resp.Body.Close()
	require.NoError(t, err)

	// Test /metrics endpoint over HTTPS
	req, err = http.NewRequestWithContext(t.Context(), http.MethodGet, "https://127.0.0.1:54322/metrics", nil)
	require.NoError(t, err)

	resp, err = tlsClient.Do(req)
	require.NoError(t, err, "HTTPS request to /metrics failed")
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	err = resp.Body.Close()
	require.NoError(t, err)

	// Verify we got some metrics
	metrics := string(body)
	require.Contains(t, metrics, "go_info", "expected Go metrics in response")

	termCh <- syscall.SIGTERM
}

func generateTestCerts(t *testing.T) (string, string) {
	t.Helper()

	tmpDir := t.TempDir()
	certFile := filepath.Join(tmpDir, "cert.pem")
	keyFile := filepath.Join(tmpDir, "key.pem")

	// Generate private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	// Create certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost"},
	}

	// Create certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	require.NoError(t, err)

	// Write cert file
	certOut, err := os.Create(certFile)
	require.NoError(t, err)
	err = pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	require.NoError(t, err)
	err = certOut.Close()
	require.NoError(t, err)

	// Write key file
	keyOut, err := os.Create(keyFile)
	require.NoError(t, err)
	err = pem.Encode(keyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})
	require.NoError(t, err)
	err = keyOut.Close()
	require.NoError(t, err)

	return certFile, keyFile
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
