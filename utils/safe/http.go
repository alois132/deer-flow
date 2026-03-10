package safe

import (
	"net/http"
	"time"
)

const (
	DefaultRetry = 3
)

type HttpClient struct {
	client     *http.Client
	retrySleep time.Duration
}

func NewHttpClient(retrySleep time.Duration) *HttpClient {
	return &HttpClient{
		client:     http.DefaultClient,
		retrySleep: retrySleep,
	}
}

func (c *HttpClient) Do(req *http.Request, retry int) (resp *http.Response, err error) {
	for i := 0; i < retry; i++ {
		resp, err = c.client.Do(req)
		if err == nil {
			return resp, nil
		}
		time.Sleep(c.retrySleep)
	}
	return nil, err
}
