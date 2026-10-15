package middleware

import (
	"mime"
	"net/http"
	"strings"

	"compress/gzip"
)

const (
	applicationJsonHeader = "application/json"
	textHTMLHeader        = "text/html"
	contentEncodingHeader = "Content-Encoding"
	contentLengthHeader   = "Content-Length"
	acceptEncodingHeader  = "Accept-Encoding"
	gzipFormat            = "gzip"
)

type compressorWriter struct {
	http.ResponseWriter
	compressor            *gzip.Writer
	isCompressionRequired bool
	wroteHeader           bool
}

func newCompressorWriter(resp http.ResponseWriter) *compressorWriter {
	return &compressorWriter{
		ResponseWriter: resp,
	}
}

func (c *compressorWriter) WriteHeader(status int) {
	c.wroteHeader = true

	if !c.isCompressionRequired {
		c.ResponseWriter.WriteHeader(status)
		return
	}

	header := c.Header().Get("Content-Type")

	mType, _, err := mime.ParseMediaType(header)
	if err != nil {
		c.ResponseWriter.WriteHeader(status)
		return
	}

	switch mType {
	case applicationJsonHeader, textHTMLHeader:
		c.compressor = gzip.NewWriter(c.ResponseWriter)
		c.Header().Add(contentEncodingHeader, gzipFormat)
		c.Header().Del(contentLengthHeader)
		c.ResponseWriter.WriteHeader(status)
	default:
		c.ResponseWriter.WriteHeader(status)
	}
}

func (c *compressorWriter) Write(data []byte) (int, error) {
	if !c.wroteHeader {
		c.WriteHeader(http.StatusOK)
	}
	if c.compressor == nil {
		return c.ResponseWriter.Write(data)
	}

	return c.compressor.Write(data)
}

func (c *compressorWriter) Close() error {
	if c.compressor == nil {
		return nil
	}

	return c.compressor.Close()
}

func Compressing() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			recorder := newCompressorWriter(resp)
			defer recorder.Close()
			format := req.Header.Get(acceptEncodingHeader)
			recorder.isCompressionRequired = strings.Contains(format, gzipFormat)

			requestCompressedFormat := req.Header.Get(contentEncodingHeader)
			if requestCompressedFormat == gzipFormat {
				decompressedBody, err := gzip.NewReader(req.Body)
				if err != nil {
					http.Error(resp, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
					return
				}
				defer decompressedBody.Close()

				req.Body = decompressedBody
			}

			next.ServeHTTP(recorder, req)

		})
	}
}
