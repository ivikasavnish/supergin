package supergin

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang/snappy"
)

// CompressionType defines the compression algorithm to use
type CompressionType string

const (
	CompressionSnappy CompressionType = "snappy"
	CompressionGzip   CompressionType = "gzip"
	CompressionNone   CompressionType = "none"
)

// CompressionConfig holds compression middleware configuration
type CompressionConfig struct {
	// Type specifies the compression algorithm (snappy, gzip, or none)
	Type CompressionType
	
	// MinSize is the minimum response size in bytes before compression is applied
	MinSize int
	
	// Level is the compression level for gzip (1-9, where 9 is best compression)
	Level int
	
	// ExcludedPaths are paths that should not be compressed
	ExcludedPaths []string
	
	// ExcludedExtensions are file extensions that should not be compressed
	ExcludedExtensions []string
}

// DefaultCompressionConfig returns a default compression configuration
func DefaultCompressionConfig() CompressionConfig {
	return CompressionConfig{
		Type:    CompressionSnappy,
		MinSize: 1024, // 1KB
		Level:   gzip.DefaultCompression,
		ExcludedExtensions: []string{
			".png", ".jpg", ".jpeg", ".gif", ".webp", ".mp4", ".avi",
			".zip", ".gz", ".tar", ".rar", ".7z",
		},
	}
}

// SnappyCompressionMiddleware creates a middleware that compresses responses using Snappy
func SnappyCompressionMiddleware(config ...CompressionConfig) gin.HandlerFunc {
	cfg := DefaultCompressionConfig()
	if len(config) > 0 {
		cfg = config[0]
	}

	return func(c *gin.Context) {
		// Check if path is excluded
		for _, path := range cfg.ExcludedPaths {
			if c.Request.URL.Path == path {
				c.Next()
				return
			}
		}

		// Check if extension is excluded
		path := c.Request.URL.Path
		for _, ext := range cfg.ExcludedExtensions {
			if strings.HasSuffix(path, ext) {
				c.Next()
				return
			}
		}

		// Create a custom response writer
		writer := &compressionWriter{
			ResponseWriter: c.Writer,
			config:         cfg,
			context:        c,
		}

		c.Writer = writer
		c.Next()

		// Compress if needed
		writer.Finish()
	}
}

// compressionWriter wraps gin.ResponseWriter to support compression
type compressionWriter struct {
	gin.ResponseWriter
	config  CompressionConfig
	context *gin.Context
	buf     bytes.Buffer
}

// Write captures the response body
func (w *compressionWriter) Write(data []byte) (int, error) {
	return w.buf.Write(data)
}

// WriteString captures the response body
func (w *compressionWriter) WriteString(s string) (int, error) {
	return w.buf.WriteString(s)
}

// Finish compresses and writes the final response
func (w *compressionWriter) Finish() {
	// If buffer is too small, don't compress
	if w.buf.Len() < w.config.MinSize {
		w.ResponseWriter.Write(w.buf.Bytes())
		return
	}

	// Get the original data
	originalData := w.buf.Bytes()

	// Compress based on type
	var compressed []byte
	var err error
	var encoding string

	switch w.config.Type {
	case CompressionSnappy:
		compressed = snappy.Encode(nil, originalData)
		encoding = "snappy"
	case CompressionGzip:
		compressed, err = compressGzip(originalData, w.config.Level)
		if err != nil {
			// If compression fails, send uncompressed
			w.ResponseWriter.Write(originalData)
			return
		}
		encoding = "gzip"
	default:
		// No compression
		w.ResponseWriter.Write(originalData)
		return
	}

	// Only use compression if it actually reduces size
	if len(compressed) < len(originalData) {
		w.ResponseWriter.Header().Set("Content-Encoding", encoding)
		w.ResponseWriter.Header().Set("X-Original-Size", strconv.Itoa(len(originalData)))
		w.ResponseWriter.Header().Set("X-Compressed-Size", strconv.Itoa(len(compressed)))
		w.ResponseWriter.Write(compressed)
	} else {
		// Compressed size is larger, send uncompressed
		w.ResponseWriter.Write(originalData)
	}
}

// compressGzip compresses data using gzip
func compressGzip(data []byte, level int) ([]byte, error) {
	var buf bytes.Buffer
	writer, err := gzip.NewWriterLevel(&buf, level)
	if err != nil {
		return nil, err
	}

	_, err = writer.Write(data)
	if err != nil {
		writer.Close()
		return nil, err
	}

	err = writer.Close()
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// DecompressionMiddleware creates a middleware that decompresses request bodies
func DecompressionMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		encoding := c.GetHeader("Content-Encoding")
		
		if encoding == "" {
			c.Next()
			return
		}

		// Read the body
		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error": "Failed to read request body",
			})
			return
		}

		var decompressed []byte

		switch encoding {
		case "snappy":
			decompressed, err = snappy.Decode(nil, bodyBytes)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
					"error": "Failed to decompress snappy data",
				})
				return
			}
		case "gzip":
			reader, err := gzip.NewReader(bytes.NewReader(bodyBytes))
			if err != nil {
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
					"error": "Failed to decompress gzip data",
				})
				return
			}
			defer reader.Close()

			decompressed, err = io.ReadAll(reader)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
					"error": "Failed to read decompressed data",
				})
				return
			}
		default:
			// Unknown encoding, pass through
			c.Next()
			return
		}

		// Replace the body with decompressed data
		c.Request.Body = io.NopCloser(bytes.NewReader(decompressed))
		c.Request.ContentLength = int64(len(decompressed))
		c.Request.Header.Del("Content-Encoding")

		c.Next()
	}
}

// CompressionMiddleware is a convenience function that adds both compression and decompression
func CompressionMiddleware(config ...CompressionConfig) gin.HandlerFunc {
	cfg := DefaultCompressionConfig()
	if len(config) > 0 {
		cfg = config[0]
	}

	// Create the decompression middleware once
	decompMiddleware := DecompressionMiddleware()
	compressionMiddleware := SnappyCompressionMiddleware(cfg)

	return func(c *gin.Context) {
		// First decompress incoming requests
		decompMiddleware(c)
		
		// Then compress outgoing responses
		if !c.IsAborted() {
			compressionMiddleware(c)
		}
	}
}
