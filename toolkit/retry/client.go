package retry

import (
	"context"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/cenkalti/backoff/v7"
	"github.com/pkg/errors"
)

const (
	firstDelay  = 1 * time.Second
	secondDelay = 3 * time.Second
	thirdDelay  = 5 * time.Second
)

var (
	ErrInvalidParam = errors.New("invalid param")

	errServiceUnavailable = errors.New("service unavailable")
)

type Doer interface {
	Do(req *http.Request) (*http.Response, error)
}

type Option func(*Client)

func WithDelays(delays ...time.Duration) Option {
	return func(client *Client) {
		client.delays = delays
	}
}

type Client struct {
	doer   Doer
	delays []time.Duration
}

func NewClient(doer Doer, options ...Option) (*Client, error) {
	if doer == nil {
		return nil, errors.Wrap(ErrInvalidParam, "new retry client. doer is nil")
	}

	client := &Client{
		doer:   doer,
		delays: []time.Duration{firstDelay, secondDelay, thirdDelay},
	}
	for _, option := range options {
		if option != nil {
			option(client)
		}
	}

	return client, nil
}

func (c *Client) Do(req *http.Request) (*http.Response, error) {
	if req.Body != nil && req.Body != http.NoBody && req.GetBody == nil {
		return c.doer.Do(req)
	}

	attempt := 0
	operation := func() (*http.Response, error) {
		attemptReq, err := c.newAttemptRequest(req, attempt)
		if err != nil {
			return nil, backoff.Permanent(err)
		}
		attempt++

		resp, err := c.doer.Do(attemptReq)
		if err != nil {
			if !c.isRetriableError(req.Context(), err) {
				return nil, backoff.Permanent(err)
			}

			return nil, err
		}
		if resp.StatusCode == http.StatusServiceUnavailable {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()

			return nil, errServiceUnavailable
		}

		return resp, nil
	}

	return backoff.Retry(
		req.Context(),
		operation,
		backoff.WithBackOff(c.newSchedule()),
		backoff.WithMaxTries(uint(len(c.delays))+1),
		backoff.WithMaxElapsedTime(0),
	)
}

func (c *Client) newAttemptRequest(req *http.Request, attempt int) (*http.Request, error) {
	if attempt == 0 || req.GetBody == nil {
		return req, nil
	}

	body, err := req.GetBody()
	if err != nil {
		return nil, errors.Wrap(err, "new attempt request. get body")
	}

	attemptReq := req.Clone(req.Context())
	attemptReq.Body = body

	return attemptReq, nil
}

func (c *Client) isRetriableError(ctx context.Context, err error) bool {
	if ctx.Err() != nil {
		return false
	}

	var netErr net.Error

	return errors.As(err, &netErr) ||
		errors.Is(err, io.EOF) ||
		errors.Is(err, io.ErrUnexpectedEOF)
}

func (c *Client) newSchedule() *schedule {
	return &schedule{delays: c.delays}
}

type schedule struct {
	delays []time.Duration
	next   int
}

func (s *schedule) NextBackOff() time.Duration {
	if s.next >= len(s.delays) {
		return backoff.Stop
	}

	delay := s.delays[s.next]
	s.next++

	return delay
}

func (s *schedule) Reset() {
	s.next = 0
}
