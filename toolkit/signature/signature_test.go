package signature_test

import (
	"encoding/base64"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/parta4ok/yandex-metrics/toolkit/signature"
)

const (
	knownKey  = "key"
	knownData = "The quick brown fox jumps over the lazy dog"
	knownHex  = "f7bc83f430538424b13298e6aa6fb143ef4d59a14946175997479dbc2d1a3cd8"
)

func newSigner(t *testing.T, key string) *signature.Signer {
	t.Helper()

	signer, err := signature.NewSigner(key)
	require.NoError(t, err)

	return signer
}

func TestNewSigner_EmptyKey(t *testing.T) {
	t.Parallel()

	signer, err := signature.NewSigner("")

	require.ErrorIs(t, err, signature.ErrInvalidParam)
	require.Nil(t, signer)
}

func TestSigner_Sign(t *testing.T) {
	t.Parallel()

	raw, err := hex.DecodeString(knownHex)
	require.NoError(t, err)

	got := newSigner(t, knownKey).Sign([]byte(knownData))

	require.Equal(t, base64.StdEncoding.EncodeToString(raw), got)
}

func TestSigner_Sign_IsDeterministicAndDependsOnInput(t *testing.T) {
	t.Parallel()

	signer := newSigner(t, knownKey)

	require.Equal(t, signer.Sign([]byte("a")), signer.Sign([]byte("a")))
	require.NotEqual(t, signer.Sign([]byte("a")), signer.Sign([]byte("b")))
	require.NotEqual(t, signer.Sign([]byte("a")), newSigner(t, "other").Sign([]byte("a")))
	require.NotEmpty(t, signer.Sign(nil))
}

func TestSigner_Verify(t *testing.T) {
	t.Parallel()

	signer := newSigner(t, knownKey)
	valid := signer.Sign([]byte(knownData))

	tests := []struct {
		name      string
		data      string
		signature string
		want      bool
	}{
		{name: "valid", data: knownData, signature: valid, want: true},
		{name: "tampered data", data: knownData + "!", signature: valid},
		{name: "wrong signature", data: knownData, signature: newSigner(t, "other").Sign([]byte(knownData))},
		{name: "empty signature", data: knownData},
		{name: "not base64", data: knownData, signature: "%%%"},
		{name: "hex instead of base64", data: knownData, signature: knownHex},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tt.want, signer.Verify([]byte(tt.data), tt.signature))
		})
	}
}

func TestSigner_Verify_EmptyData(t *testing.T) {
	t.Parallel()

	signer := newSigner(t, knownKey)

	require.True(t, signer.Verify(nil, signer.Sign(nil)))
}

func TestHeader(t *testing.T) {
	t.Parallel()

	require.Equal(t, "HashSHA256", signature.Header)
}
