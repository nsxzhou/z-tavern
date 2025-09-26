package speech

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
)

// CompressPayload 压缩payload
func CompressPayload(data []byte, method CompressionMethod) ([]byte, error) {
	switch method {
	case NoCompression:
		return data, nil
	case GzipCompression:
		return compressGzip(data)
	default:
		return nil, fmt.Errorf("unsupported compression method: %d", method)
	}
}

// DecompressPayload 解压缩payload
func DecompressPayload(data []byte, method CompressionMethod) ([]byte, error) {
	switch method {
	case NoCompression:
		return data, nil
	case GzipCompression:
		return decompressGzip(data)
	default:
		return nil, fmt.Errorf("unsupported compression method: %d", method)
	}
}

// compressGzip 使用gzip压缩数据
func compressGzip(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)

	if _, err := writer.Write(data); err != nil {
		writer.Close()
		return nil, fmt.Errorf("gzip write failed: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("gzip close failed: %w", err)
	}

	return buf.Bytes(), nil
}

// decompressGzip 使用gzip解压缩数据
func decompressGzip(data []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("gzip reader creation failed: %w", err)
	}
	defer reader.Close()

	result, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("gzip read failed: %w", err)
	}

	return result, nil
}