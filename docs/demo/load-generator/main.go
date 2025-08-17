package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"math/rand"
	"net/http"
	"os"
	"sync"
	"time"
)

// ThrottledReader wraps an io.Reader to introduce artificial latency.
type ThrottledReader struct {
	reader          io.Reader
	maxBytesPerRead int
	minDelay        time.Duration
	maxDelay        time.Duration
}

// NewThrottledReader creates a reader that introduces random delays and limits bytes per read.
func NewThrottledReader(reader io.Reader, maxBytesPerRead int, minDelay, maxDelay time.Duration) *ThrottledReader {
	return &ThrottledReader{
		reader:          reader,
		maxBytesPerRead: maxBytesPerRead,
		minDelay:        minDelay,
		maxDelay:        maxDelay,
	}
}

func (t *ThrottledReader) Read(p []byte) (n int, err error) {
	// Limit the number of bytes we read at once to simulate slow network
	if len(p) > t.maxBytesPerRead {
		p = p[:t.maxBytesPerRead]
	}

	// Add random delay before reading to simulate network latency
	if t.maxDelay > 0 {
		delayRange := t.maxDelay - t.minDelay

		var randomDelay time.Duration

		if delayRange > 0 {
			randomDelay = t.minDelay + time.Duration(rand.Int63n(int64(delayRange)))
		} else {
			// If delayRange is 0 or negative, just use minDelay
			randomDelay = t.minDelay
		}

		time.Sleep(randomDelay)
	}

	return t.reader.Read(p)
}

// StreamingRandomReader generates random content on-demand.
type StreamingRandomReader struct {
	charset   []byte
	buffer    []byte
	remaining int
	blockSize int
	bufPos    int
}

// NewStreamingRandomReader creates a new streaming random content generator.
func NewStreamingRandomReader(size int) *StreamingRandomReader {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	const blockSize = 8192 // 8KB buffer

	return &StreamingRandomReader{
		remaining: size,
		blockSize: blockSize,
		charset:   []byte(charset),
		buffer:    make([]byte, blockSize),
		bufPos:    blockSize, // Force initial buffer generation
	}
}

func (r *StreamingRandomReader) Read(p []byte) (n int, err error) {
	if r.remaining <= 0 {
		return 0, io.EOF
	}

	bytesToRead := len(p)
	if bytesToRead > r.remaining {
		bytesToRead = r.remaining
	}

	totalRead := 0

	for totalRead < bytesToRead {
		// Refill buffer if needed
		if r.bufPos >= len(r.buffer) {
			r.generateBuffer()
			r.bufPos = 0
		}

		// Copy from buffer to output
		bytesFromBuffer := len(r.buffer) - r.bufPos

		bytesNeeded := bytesToRead - totalRead
		if bytesFromBuffer > bytesNeeded {
			bytesFromBuffer = bytesNeeded
		}

		copy(p[totalRead:], r.buffer[r.bufPos:r.bufPos+bytesFromBuffer])
		r.bufPos += bytesFromBuffer
		totalRead += bytesFromBuffer
		r.remaining -= bytesFromBuffer
	}

	return totalRead, nil
}

func (r *StreamingRandomReader) generateBuffer() {
	for i := range r.buffer {
		r.buffer[i] = r.charset[rand.Intn(len(r.charset))]
	}
}

// StreamingJSONReader generates JSON content on-demand.
type StreamingJSONReader struct {
	dataReader *StreamingRandomReader
	headerBuf  []byte
	footerBuf  []byte
	state      int
	dataSize   int
	dataRead   int
	headerPos  int
	footerPos  int
}

func NewStreamingJSONReader(totalSize int) *StreamingJSONReader {
	// Pre-generate header and footer to avoid allocations during Read()
	header := fmt.Sprintf(`{"id": %d, "data": "`, rand.Intn(10000))
	footer := fmt.Sprintf(`", "timestamp": %d}`, time.Now().Unix())

	// Calculate exact data size based on actual header/footer lengths
	headerLen := len(header)
	footerLen := len(footer)

	dataSize := totalSize - headerLen - footerLen
	if dataSize < 1 {
		dataSize = 1
		// Recalculate total size if we had to adjust dataSize
		totalSize = headerLen + dataSize + footerLen
	}

	return &StreamingJSONReader{
		state:      0,
		dataSize:   dataSize,
		dataReader: NewStreamingRandomReader(dataSize),
		headerBuf:  []byte(header),
		footerBuf:  []byte(footer),
	}
}

// Add ActualSize method to StreamingJSONReader.
func (r *StreamingJSONReader) ActualSize() int64 {
	return int64(len(r.headerBuf) + r.dataSize + len(r.footerBuf))
}

func (r *StreamingJSONReader) Read(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}

	switch r.state {
	case 0: // JSON header
		available := len(r.headerBuf) - r.headerPos

		toCopy := len(p)
		if toCopy > available {
			toCopy = available
		}

		copy(p, r.headerBuf[r.headerPos:r.headerPos+toCopy])
		r.headerPos += toCopy

		if r.headerPos >= len(r.headerBuf) {
			r.state = 1 // Move to data state
		}

		return toCopy, nil

	case 1: // JSON data content
		n, err = r.dataReader.Read(p)

		r.dataRead += n
		if err == io.EOF {
			r.state = 2
			err = nil // Continue to footer
		}

		return n, err

	case 2: // JSON footer
		available := len(r.footerBuf) - r.footerPos

		toCopy := len(p)
		if toCopy > available {
			toCopy = available
		}

		copy(p, r.footerBuf[r.footerPos:r.footerPos+toCopy])
		r.footerPos += toCopy

		if r.footerPos >= len(r.footerBuf) {
			r.state = 3 // Done
			return toCopy, io.EOF
		}

		return toCopy, nil

	default: // Done
		return 0, io.EOF
	}
}

var baseURL = os.Getenv("BASE_URL")

// Dynamic load parameters that change every 5 minutes.
type LoadParameters struct {
	lastUpdate        time.Time
	errorRate         float64
	meanDelayMs       int
	delaySpreadFactor float64
	meanSizeBytes     int
	sizeSpreadFactor  float64
	requestRateHz     float64
	mu                sync.RWMutex
}

var loadParams = &LoadParameters{
	errorRate:         0.05,  // Start with 5% error rate
	meanDelayMs:       100,   // Start with 100ms mean delay
	delaySpreadFactor: 2.0,   // 2x spread
	meanSizeBytes:     50000, // Start with 50KB mean size
	sizeSpreadFactor:  3.0,   // 3x spread
	requestRateHz:     5.0,   // Start with 5 requests per second
	lastUpdate:        time.Now(),
}

// Update load parameters every 5 minutes.
func updateLoadParameters() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			loadParams.mu.Lock()

			// Update error rate (0% to 80%)
			loadParams.errorRate = rand.Float64() * 0.8

			// Update mean delay (50ms to 2000ms)
			loadParams.meanDelayMs = rand.Intn(1950) + 50

			// Update delay spread factor (1.5x to 5x)
			loadParams.delaySpreadFactor = 1.5 + rand.Float64()*3.5

			// Update mean size (100 bytes to 10MB)
			loadParams.meanSizeBytes = rand.Intn(10485700-100) + 100

			// Update size spread factor (2x to 8x)
			loadParams.sizeSpreadFactor = 2.0 + rand.Float64()*6.0

			// Update request rate (1 to 100 requests per second)
			loadParams.requestRateHz = rand.Float64()*99 + 1

			loadParams.lastUpdate = time.Now()

			log.Printf("Load parameters updated: ErrorRate=%.1f%%, MeanDelay=%dms (spread=%.1fx), MeanSize=%d bytes (spread=%.1fx), RequestRate=%.1f req/s",
				loadParams.errorRate*100,
				loadParams.meanDelayMs,
				loadParams.delaySpreadFactor,
				loadParams.meanSizeBytes,
				loadParams.sizeSpreadFactor,
				loadParams.requestRateHz,
			)

			loadParams.mu.Unlock()
		}
	}
}

// Generate delay with current parameters.
func generateDynamicDelay() time.Duration {
	loadParams.mu.RLock()
	defer loadParams.mu.RUnlock()

	mean := float64(loadParams.meanDelayMs)
	spread := mean * loadParams.delaySpreadFactor

	// Use exponential distribution for more realistic delay patterns
	delay := math.Abs(rand.NormFloat64()*spread/3) + mean*0.1
	if delay > mean*10 { // Cap at 10x mean to avoid extreme outliers
		delay = mean * 10
	}

	return time.Duration(delay) * time.Millisecond
}

// Generate size with current parameters.
func generateDynamicSize(baseSize int) int {
	loadParams.mu.RLock()
	defer loadParams.mu.RUnlock()

	if baseSize <= 0 {
		baseSize = loadParams.meanSizeBytes
	}

	mean := float64(baseSize)
	spread := mean * loadParams.sizeSpreadFactor

	// Use log-normal distribution for size variation
	size := math.Abs(rand.NormFloat64()*spread/4) + mean*0.2
	if size > mean*15 { // Cap at 15x mean
		size = mean * 15
	}

	result := int(size)
	if result < 1 {
		result = 1
	}

	return result
}

// Check if this request should be a 500 error.
func shouldReturnError() bool {
	loadParams.mu.RLock()
	defer loadParams.mu.RUnlock()

	return rand.Float64() < loadParams.errorRate
}

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

// Get current request rate.
func getCurrentRequestRate() float64 {
	loadParams.mu.RLock()
	defer loadParams.mu.RUnlock()

	return loadParams.requestRateHz
}

func main() {
	log.Println("Starting enhanced load generator with dynamic parameters...")

	// Start the parameter updater in background
	go updateLoadParameters()

	// Log initial parameters
	log.Printf("Initial parameters: ErrorRate=%.1f%%, MeanDelay=%dms, MeanSize=%d bytes, RequestRate=%.1f req/s",
		loadParams.errorRate*100,
		loadParams.meanDelayMs,
		loadParams.meanSizeBytes,
		loadParams.requestRateHz,
	)

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 60 * time.Second, // Increased timeout for delay testing
	}

	// Rate-controlled request generation
	for {
		go makeRandomRequest(client)

		// Calculate delay based on current request rate
		rate := getCurrentRequestRate()
		baseDelay := time.Duration(1000.0/rate) * time.Millisecond

		// Add some jitter to make it more realistic (±20%)
		jitter := 1.0 + (rand.Float64()-0.5)*0.4
		delay := time.Duration(float64(baseDelay) * jitter)

		time.Sleep(delay)
	}
}

func makeRandomRequest(client *http.Client) {
	var (
		url    string
		method string
		body   io.Reader
	)

	// Force 500 error if error rate dictates it
	if shouldReturnError() {
		url = baseURL + "/direct/500"
		method = http.MethodGet
	} else {
		// Randomly choose endpoint type with weighted distribution
		choice := rand.Intn(100)

		switch {
		case choice < 25:
			// Direct endpoints - always GET
			url = baseURL + directEndpoints[rand.Intn(len(directEndpoints))]
			method = http.MethodGet
		case choice < 45:
			// httpbin/bytes/{n} endpoint - dynamic response sizes
			n := generateDynamicSize(loadParams.meanSizeBytes)
			url = fmt.Sprintf("%s/httpbin/bytes/%d", baseURL, n)
			method = http.MethodGet
		case choice < 65:
			// httpbin/delay/{delay} endpoint - dynamic delays
			delaySeconds := int(generateDynamicDelay().Seconds())
			if delaySeconds < 1 {
				delaySeconds = 1
			}

			if delaySeconds > 30 { // Cap at 30 seconds
				delaySeconds = 30
			}

			url = fmt.Sprintf("%s/httpbin/delay/%d", baseURL, delaySeconds)
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
	}

	// Create request
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		log.Printf("Error creating request: %v", err)
		return
	}

	// Set Content-Length for sized readers
	if sizedReader, ok := body.(*SizedReader); ok {
		req.ContentLength = sizedReader.Size()
	}

	// Add random headers
	addRandomHeaders(req)

	// Make request with better error handling
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

	// Read response body with improved error handling and throttling
	// Generate different throttling parameters for response reading
	respMaxBytes, respMinDelay, respMaxDelay := generateRandomThrottling()
	throttledRespBody := NewThrottledReader(resp.Body, respMaxBytes, respMinDelay, respMaxDelay)

	bytesRead, err := io.Copy(io.Discard, throttledRespBody)
	if err != nil {
		// Check if it's just an EOF which might be expected for some endpoints
		if err == io.EOF {
			log.Printf("Request completed with EOF: %s %s - Status: %d, Read: %d bytes (took %v)", method, url, resp.StatusCode, bytesRead, duration)
		} else if errors.Is(err, io.ErrUnexpectedEOF) {
			log.Printf("Request completed with unexpected EOF: %s %s - Status: %d, Read: %d bytes (took %v)", method, url, resp.StatusCode, bytesRead, duration)
		} else {
			log.Printf("Error reading response body: %s %s - Status: %d, Error: %v, Read: %d bytes (took %v)", method, url, resp.StatusCode, err, bytesRead, duration)
		}

		return
	}

	// Update duration to include throttled response reading time
	totalDuration := time.Since(start)
	log.Printf("Request completed: %s %s - Status: %d, Read: %d bytes (took %v)", method, url, resp.StatusCode, bytesRead, totalDuration)
}

// SizedReader wraps streaming readers to provide Content-Length.
type SizedReader struct {
	reader io.Reader
	size   int64
}

func NewSizedReader(reader io.Reader, size int64) *SizedReader {
	return &SizedReader{
		reader: reader,
		size:   size,
	}
}

func (r *SizedReader) Read(p []byte) (n int, err error) {
	return r.reader.Read(p)
}

func (r *SizedReader) Size() int64 {
	return r.size
}

// StreamingFormReader generates form data on-demand.
type StreamingFormReader struct {
	reader     *StreamingRandomReader
	sepBuf     []byte
	prefixBuf  []byte
	state      int
	fieldSize  int
	sepPos     int
	prefixPos  int
	totalSize  int
	actualSize int
}

func generateStreamingFormData(targetSize int) io.Reader {
	// Pre-calculate separator to know its exact size
	separator := fmt.Sprintf("&field2=%d&field3=", rand.Intn(1000))
	prefix := "field1="

	// Calculate field size based on actual component sizes
	prefixLen := len(prefix)
	sepLen := len(separator)
	overhead := prefixLen + sepLen

	// Split remaining size between two fields
	remainingSize := targetSize - overhead
	if remainingSize < 2 {
		remainingSize = 2
	}

	fieldSize := remainingSize / 2

	actualTotalSize := prefixLen + fieldSize + sepLen + fieldSize

	return &StreamingFormReader{
		state:      0,
		fieldSize:  fieldSize,
		prefixBuf:  []byte(prefix),
		sepBuf:     []byte(separator),
		totalSize:  targetSize,
		actualSize: actualTotalSize,
	}
}

func (r *StreamingFormReader) Read(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}

	switch r.state {
	case 0: // field1= prefix
		available := len(r.prefixBuf) - r.prefixPos

		toCopy := len(p)
		if toCopy > available {
			toCopy = available
		}

		copy(p, r.prefixBuf[r.prefixPos:r.prefixPos+toCopy])
		r.prefixPos += toCopy

		if r.prefixPos >= len(r.prefixBuf) {
			r.reader = NewStreamingRandomReader(r.fieldSize)
			r.state = 1
		}

		return toCopy, nil

	case 1: // field1 data
		n, err = r.reader.Read(p)
		if err == io.EOF {
			r.state = 2
			err = nil
		}

		return n, err

	case 2: // separator
		available := len(r.sepBuf) - r.sepPos

		toCopy := len(p)
		if toCopy > available {
			toCopy = available
		}

		copy(p, r.sepBuf[r.sepPos:r.sepPos+toCopy])
		r.sepPos += toCopy

		if r.sepPos >= len(r.sepBuf) {
			r.reader = NewStreamingRandomReader(r.fieldSize)
			r.state = 3
		}

		return toCopy, nil

	case 3: // field3 data
		n, err = r.reader.Read(p)
		if err == io.EOF {
			r.state = 4
		}

		return n, err

	default:
		return 0, io.EOF
	}
}

// StreamingFormReader implements SizedReaderInterface.
func (r *StreamingFormReader) ActualSize() int64 {
	return int64(r.actualSize)
}

// Generate random throttling parameters for network simulation.
func generateRandomThrottling() (maxBytesPerRead int, minDelay, maxDelay time.Duration) {
	// Random bandwidth simulation (100 bytes to 64KB per read)
	maxBytesPerRead = 100 + rand.Intn(65436)

	// Random network latency (0-50ms base delay, 0-200ms max delay)
	minDelay = time.Duration(rand.Intn(50)) * time.Millisecond
	maxDelay = minDelay + time.Duration(rand.Intn(200))*time.Millisecond

	return maxBytesPerRead, minDelay, maxDelay
}

func generateRandomBody() io.Reader {
	// Use dynamic size generation
	size := generateDynamicSize(0) // Use current mean size

	// Choose random content type
	contentTypes := []string{
		"application/json",
		"text/plain",
		"application/x-www-form-urlencoded",
		"application/octet-stream",
	}

	contentType := contentTypes[rand.Intn(len(contentTypes))]

	var (
		reader     io.Reader
		actualSize int64
	)

	switch contentType {
	case "application/json":
		jsonReader := NewStreamingJSONReader(size)
		reader = jsonReader
		actualSize = jsonReader.ActualSize()
	case "text/plain":
		reader = NewStreamingRandomReader(size)
		actualSize = int64(size)
	case "application/x-www-form-urlencoded":
		formReader := generateStreamingFormData(size).(*StreamingFormReader)
		reader = formReader
		actualSize = formReader.ActualSize()
	default:
		reader = NewStreamingRandomReader(size)
		actualSize = int64(size)
	}

	// Add random throttling to simulate slow upload speeds
	maxBytes, minDelay, maxDelay := generateRandomThrottling()
	throttledReader := NewThrottledReader(reader, maxBytes, minDelay, maxDelay)

	return NewSizedReader(throttledReader, actualSize)
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
