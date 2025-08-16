package main

import (
	"bytes"
	crypt "crypto/rand"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"
)

var baseURL = os.Getenv("BASE_URL")

var (
	directEndpoints = []string{
		"/direct/200",
		"/direct/204",
		"/direct/404",
		"/direct/500",
	}

	httpMethods = []string{
		http.MethodGet,
		http.MethodPost,
		http.MethodPut,
		http.MethodPatch,
		http.MethodDelete,
		http.MethodHead,
		http.MethodOptions,
	}

	httpbinEndpoints = []string{
		"/httpbin/",
		"/httpbin/get",
		"/httpbin/post",
		"/httpbin/put",
		"/httpbin/patch",
		"/httpbin/delete",
		"/httpbin/status/200",
		"/httpbin/status/201",
		"/httpbin/status/400",
		"/httpbin/status/404",
		"/httpbin/status/500",
		"/httpbin/headers",
		"/httpbin/ip",
		"/httpbin/user-agent",
		"/httpbin/json",
		"/httpbin/html",
		"/httpbin/xml",
		"/httpbin/uuid",
		"/httpbin/base64/encode",
		"/httpbin/base64/decode",
	}
)

func main() {
	log.Println("Starting load generator...")

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Generate load continuously
	for {
		go makeRandomRequest(client)

		// Random delay between requests (10ms to 500ms)
		delay := time.Duration(rand.Intn(490)+10) * time.Millisecond
		time.Sleep(delay)
	}
}

func makeRandomRequest(client *http.Client) {
	var (
		url    string
		method string
		body   io.Reader
	)

	// Randomly choose endpoint type

	switch rand.Intn(4) {
	case 0:
		// Direct endpoints - always GET
		url = baseURL + directEndpoints[rand.Intn(len(directEndpoints))]
		method = http.MethodGet
	case 1:
		// httpbin/bytes/{n} endpoint - random response sizes
		n := rand.Intn(10000) + 1 // 1 to 10000 bytes
		url = fmt.Sprintf("%s/httpbin/bytes/%d", baseURL, n)
		method = http.MethodGet
	case 2:
		// httpbin/delay/{delay} endpoint
		delay := rand.Intn(10) + 1 // 1 to 10 seconds
		url = fmt.Sprintf("%s/httpbin/delay/%d", baseURL, delay)
		method = http.MethodGet
	default:
		// Other httpbin endpoints
		url = baseURL + httpbinEndpoints[rand.Intn(len(httpbinEndpoints))]
		method = httpMethods[rand.Intn(len(httpMethods))]

		// Add request body for POST, PUT, PATCH methods
		if method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch {
			body = generateRandomBody()
		}
	}

	// Create request
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		log.Printf("Error creating request: %v", err)
		return
	}

	// Add random headers
	addRandomHeaders(req)

	// Make request
	start := time.Now()
	resp, err := client.Do(req)
	duration := time.Since(start)

	if err != nil {
		log.Printf("Request failed: %s %s - Error: %v (took %v)", method, url, err, duration)
		return
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("Error closing response body: %v", err)
		}
	}()

	// Read response body to ensure complete request
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		log.Printf("Error reading response body: %v", err)
	}

	log.Printf("Request completed: %s %s - Status: %d (took %v)", method, url, resp.StatusCode, duration)
}

func generateRandomBody() io.Reader {
	// Generate random body size between 1 byte and 5KB
	size := rand.Intn(5120) + 1

	// Choose random content type
	contentTypes := []string{
		"application/json",
		"text/plain",
		"application/x-www-form-urlencoded",
		"application/octet-stream",
	}

	contentType := contentTypes[rand.Intn(len(contentTypes))]

	var bodyData []byte

	switch contentType {
	case "application/json":
		bodyData = generateRandomJSON(size)
	case "text/plain":
		bodyData = generateRandomText(size)
	case "application/x-www-form-urlencoded":
		bodyData = generateRandomFormData(size)
	default:
		bodyData = generateRandomBytes(size)
	}

	return bytes.NewReader(bodyData)
}

func generateRandomJSON(targetSize int) []byte {
	// Generate a simple JSON object with random data
	dataSize := targetSize - 50 // Reserve space for JSON structure
	if dataSize < 1 {
		dataSize = 1
	}

	json := fmt.Sprintf(`{"id": %d, "data": "%s", "timestamp": %d}`,
		rand.Intn(10000),
		generateRandomString(dataSize),
		time.Now().Unix(),
	)

	return []byte(json)
}

func generateRandomText(size int) []byte {
	return []byte(generateRandomString(size))
}

func generateRandomFormData(targetSize int) []byte {
	fieldSize := targetSize / 3
	if fieldSize < 1 {
		fieldSize = 1
	}

	form := fmt.Sprintf("field1=%s&field2=%d&field3=%s",
		generateRandomString(fieldSize),
		rand.Intn(1000),
		generateRandomString(fieldSize),
	)

	return []byte(form)
}

func generateRandomBytes(size int) []byte {
	data := make([]byte, size)
	if _, err := crypt.Read(data); err != nil {
		log.Printf("Error generating random bytes: %v", err)
		// Fallback to pseudo-random data
		for i := range data {
			data[i] = byte(rand.Intn(256))
		}
	}

	return data
}

func generateRandomString(length int) string {
	if length <= 0 {
		return ""
	}

	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	result := make([]byte, length)
	for i := range result {
		result[i] = charset[rand.Intn(len(charset))]
	}

	return string(result)
}

func addRandomHeaders(req *http.Request) {
	// Add Content-Type for requests with body
	if req.Body != nil {
		contentTypes := []string{
			"application/json",
			"text/plain",
			"application/x-www-form-urlencoded",
			"application/octet-stream",
		}
		req.Header.Set("Content-Type", contentTypes[rand.Intn(len(contentTypes))])
	}

	// Add random User-Agent
	userAgents := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36",
		"curl/7.68.0",
		"wget/1.20.3",
		"PostmanRuntime/7.28.0",
	}
	req.Header.Set("User-Agent", userAgents[rand.Intn(len(userAgents))])

	// Randomly add other headers
	if rand.Intn(2) == 0 {
		req.Header.Set("Accept", "application/json")
	}

	if rand.Intn(2) == 0 {
		req.Header.Set("Accept-Encoding", "gzip, deflate")
	}

	if rand.Intn(3) == 0 {
		req.Header.Set("X-Request-ID", fmt.Sprintf("req-%d", rand.Intn(100000)))
	}
}
