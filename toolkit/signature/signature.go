package signature

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"

	"github.com/pkg/errors"
)

const Header = "HashSHA256"

var ErrInvalidParam = errors.New("invalid param")

type Signer struct {
	key []byte
}

func NewSigner(key string) (*Signer, error) {
	if key == "" {
		return nil, errors.Wrap(ErrInvalidParam, "new signer. key is empty")
	}

	return &Signer{key: []byte(key)}, nil
}

func (s *Signer) Sign(data []byte) string {
	mac := hmac.New(sha256.New, s.key)
	mac.Write(data)

	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func (s *Signer) Verify(data []byte, signature string) bool {
	received, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return false
	}

	mac := hmac.New(sha256.New, s.key)
	mac.Write(data)

	return hmac.Equal(mac.Sum(nil), received)
}
