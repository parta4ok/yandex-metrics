package middleware

import (
	"bytes"
	"io"
	"net/http"

	"github.com/pkg/errors"

	"github.com/parta4ok/yandex-metrics/toolkit/signature"
)

const MaxSignedBodyBytes = 1 << 20

type signingWriter struct {
	http.ResponseWriter
	signer      *signature.Signer
	body        bytes.Buffer
	status      int
	wroteHeader bool
}

func newSigningWriter(resp http.ResponseWriter, signer *signature.Signer) *signingWriter {
	return &signingWriter{
		ResponseWriter: resp,
		signer:         signer,
		status:         http.StatusOK,
	}
}

func (w *signingWriter) WriteHeader(status int) {
	if !w.wroteHeader {
		w.status = status
		w.wroteHeader = true
	}
}

func (w *signingWriter) Write(data []byte) (int, error) {
	w.wroteHeader = true

	return w.body.Write(data)
}

func (w *signingWriter) flush() {
	w.Header().Set(signature.Header, w.signer.Sign(w.body.Bytes()))
	w.ResponseWriter.WriteHeader(w.status)
	_, _ = w.ResponseWriter.Write(w.body.Bytes())
}

type signing struct {
	signer *signature.Signer
}

func (s *signing) verifyRequest(resp http.ResponseWriter, req *http.Request) int {
	values, ok := req.Header[http.CanonicalHeaderKey(signature.Header)]
	if !ok {
		return http.StatusOK
	}

	var body []byte
	if req.Body != nil {
		var err error
		if body, err = io.ReadAll(http.MaxBytesReader(resp, req.Body, MaxSignedBodyBytes)); err != nil {
			var tooLarge *http.MaxBytesError
			if errors.As(err, &tooLarge) {
				return http.StatusRequestEntityTooLarge
			}

			return http.StatusBadRequest
		}
	}
	req.Body = io.NopCloser(bytes.NewReader(body))

	if !s.signer.Verify(body, values[0]) {
		return http.StatusBadRequest
	}

	return http.StatusOK
}

func (s *signing) handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		writer := newSigningWriter(resp, s.signer)

		if status := s.verifyRequest(resp, req); status != http.StatusOK {
			http.Error(writer, http.StatusText(status), status)
		} else {
			next.ServeHTTP(writer, req)
		}

		writer.flush()
	})
}

func Signing(signer *signature.Signer) func(http.Handler) http.Handler {
	if signer == nil {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	return (&signing{signer: signer}).handle
}
