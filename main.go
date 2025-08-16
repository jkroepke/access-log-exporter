package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"log"
	"math"
	mathrand "math/rand"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const (
	baseURL     = "http://localhost:8090"
	minRPM      = 2
	maxRPM      = 120
	minBodySize = 1024    // 1KB
	maxBodySize = 1048576 // 1MB
)

var (
	endpoints = []string{"/200", "/204", "/404", "/500", "/proxy/200", "/proxy/204", "/proxy/404", "/proxy/500"}
	methods   = []string{"GET", "POST", "PUT", "DELETE", "HEAD", "PATCH"}
	client    = &http.Client{
		Timeout: 30 * time.Second,
	}
)

type RequestStats struct {
	Endpoint   string
	Method     string
	StatusCode int
	Duration   time.Duration
	BodySize   int
	Timestamp  time.Time
}

func main() {
	log.Println("🚀 Starting HTTP Load Generator")
	log.Printf("📊 Target: %s", baseURL)
	log.Printf("⚡ Request rate: %d-%d RPM (random)", minRPM, maxRPM)
	log.Printf("🎯 Endpoints: %v", endpoints)
	log.Printf("🔧 Methods: %v", methods)
	log.Println("📈 Perfect for dashboard data generation!")

	// Seed random number generator
	mathrand.Seed(time.Now().UnixNano())

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("\n🛑 Shutdown signal received, stopping load generator...")
		cancel()
	}()

	// Start load generation
	generateLoad(ctx)
	log.Println("👋 Load generator stopped")
}

func generateLoad(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	var requestCount int64
	startTime := time.Now()

	for {
		select {
		case <-ctx.Done():
			duration := time.Since(startTime)
			avgRPS := float64(requestCount) / duration.Seconds()
			log.Printf("📊 Final Stats: %d requests in %v (avg %.1f RPS)",
				requestCount, duration.Truncate(time.Second), avgRPS)
			return
		case <-ticker.C:
			// Calculate dynamic request rate (requests per minute)
			currentRPM := mathrand.Intn(maxRPM-minRPM+1) + minRPM

			// Convert to requests per second with some randomness
			baseRPS := float64(currentRPM) / 60.0
			jitter := (mathrand.Float64() - 0.5) * 0.4 // ±20% jitter
			actualRPS := math.Max(0.1, baseRPS*(1+jitter))

			// Calculate interval between requests
			interval := time.Duration(float64(time.Second) / actualRPS)

			// Send requests at calculated rate
			go sendBurstRequests(ctx, interval, &requestCount)
		}
	}
}

func sendBurstRequests(ctx context.Context, interval time.Duration, requestCount *int64) {
	// Send 1-3 requests in this burst
	burstSize := mathrand.Intn(3) + 1

	for i := 0; i < burstSize; i++ {
		select {
		case <-ctx.Done():
			return
		default:
			go func() {
				stats := sendRandomRequest()
				*requestCount++

				// Log every 10th request with more details
				if *requestCount%10 == 0 {
					log.Printf("📊 [%d] %s %s -> %d (%v) [%s]",
						*requestCount, stats.Method, stats.Endpoint,
						stats.StatusCode, stats.Duration.Truncate(time.Millisecond),
						formatBytes(stats.BodySize))
				}
			}()

			if i < burstSize-1 {
				time.Sleep(interval / time.Duration(burstSize))
			}
		}
	}
}

func sendRandomRequest() RequestStats {
	endpoint := endpoints[mathrand.Intn(len(endpoints))]
	method := methods[mathrand.Intn(len(methods))]

	// Create request with random body for POST/PUT
	var body io.Reader
	var bodySize int

	if method == "POST" || method == "PUT" || method == "PATCH" {
		bodySize = generateRandomBodySize()
		body = bytes.NewReader(generateRandomBody(bodySize))
	}

	url := baseURL + endpoint
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		log.Printf("❌ Error creating request: %v", err)
		return RequestStats{}
	}

	// Add some realistic headers
	req.Header.Set("User-Agent", generateRandomUserAgent())
	req.Header.Set("Accept", "application/json, text/plain, */*")

	if body != nil {
		req.Header.Set("Content-Type", generateRandomContentType())
	}

	// Add random custom headers for more realistic traffic
	if mathrand.Float32() < 0.3 { // 30% chance
		req.Header.Set("X-Request-ID", generateRandomID())
	}
	if mathrand.Float32() < 0.2 { // 20% chance
		req.Header.Set("X-Correlation-ID", generateRandomID())
	}

	start := time.Now()
	resp, err := client.Do(req)
	duration := time.Since(start)

	stats := RequestStats{
		Endpoint:  endpoint,
		Method:    method,
		Duration:  duration,
		BodySize:  bodySize,
		Timestamp: start,
	}

	if err != nil {
		log.Printf("❌ Request failed: %s %s - %v", method, endpoint, err)
		return stats
	}
	defer resp.Body.Close()

	stats.StatusCode = resp.StatusCode

	// Consume response body to simulate real usage
	io.Copy(io.Discard, resp.Body)

	return stats
}

func generateRandomBodySize() int {
	// Generate size with bias towards smaller payloads (more realistic)
	random := mathrand.Float64()

	// 60% small (1KB-10KB), 30% medium (10KB-100KB), 10% large (100KB-1MB)
	if random < 0.6 {
		return mathrand.Intn(10*1024-minBodySize) + minBodySize
	} else if random < 0.9 {
		return mathrand.Intn(100*1024-10*1024) + 10*1024
	} else {
		return mathrand.Intn(maxBodySize-100*1024) + 100*1024
	}
}

func generateRandomBody(size int) []byte {
	// Create semi-realistic JSON-like data
	patterns := []string{
		`{"id": %d, "data": "%s", "timestamp": "%s"}`,
		`{"user_id": %d, "action": "%s", "payload": "%s"}`,
		`{"event": "%s", "data": %s, "metadata": {"size": %d}}`,
	}

	pattern := patterns[mathrand.Intn(len(patterns))]

	// Generate base content
	baseContent := fmt.Sprintf(pattern,
		mathrand.Intn(100000),
		generateRandomString(20),
		time.Now().Format(time.RFC3339),
	)

	// Pad to desired size with realistic data
	if len(baseContent) >= size {
		return []byte(baseContent[:size])
	}

	result := make([]byte, size)
	copy(result, baseContent)

	// Fill remaining space with random data
	padding := make([]byte, size-len(baseContent))
	rand.Read(padding)

	// Make it more text-like (printable characters)
	for i := range padding {
		padding[i] = byte(32 + (padding[i] % 95)) // ASCII printable range
	}

	copy(result[len(baseContent):], padding)
	return result
}

func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[mathrand.Intn(len(charset))]
	}
	return string(b)
}

func generateRandomUserAgent() string {
	userAgents := []string{
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Safari/605.1.15",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"curl/7.68.0",
		"Go-http-client/1.1",
		"PostmanRuntime/7.32.3",
	}
	return userAgents[mathrand.Intn(len(userAgents))]
}

func generateRandomContentType() string {
	contentTypes := []string{
		"application/json",
		"application/x-www-form-urlencoded",
		"text/plain",
		"application/xml",
		"multipart/form-data",
	}
	return contentTypes[mathrand.Intn(len(contentTypes))]
}

func generateRandomID() string {
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		mathrand.Uint32(),
		mathrand.Uint32()&0xffff,
		mathrand.Uint32()&0xffff,
		mathrand.Uint32()&0xffff,
		mathrand.Uint64()&0xffffffffffff,
	)
}

func formatBytes(bytes int) string {
	if bytes == 0 {
		return "0B"
	}

	units := []string{"B", "KB", "MB"}
	i := 0
	size := float64(bytes)

	for size >= 1024 && i < len(units)-1 {
		size /= 1024
		i++
	}

	return fmt.Sprintf("%.1f%s", size, units[i])
}
