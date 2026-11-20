package middleware

import (
	"bytes"
	"io"
	"net/http"

	"github.com/parta4ok/yandex-metrics/toolkit/signature"
)

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

func (s *signing) verifyRequest(req *http.Request) bool {
	received := req.Header.Get(signature.Header)
	if received == "" {
		return true
	}

	var body []byte
	if req.Body != nil {
		var err error
		if body, err = io.ReadAll(req.Body); err != nil {
			return false
		}
	}
	req.Body = io.NopCloser(bytes.NewReader(body))

	return s.signer.Verify(body, received)
}

func (s *signing) handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		writer := newSigningWriter(resp, s.signer)
		defer writer.flush()

		if !s.verifyRequest(req) {
			http.Error(writer, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		next.ServeHTTP(writer, req)
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
