package download

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/aquaproj/aqua/v2/pkg/github"
	"github.com/sirupsen/logrus"
	"github.com/suzuki-shunsuke/logrus-error/logerr"
)

type HTTPDownloader interface {
	Download(ctx context.Context, u string) (io.ReadCloser, int64, error)
}

func NewHTTPDownloader(logE *logrus.Entry, httpClient *http.Client) HTTPDownloader {
	return &httpDownloader{
		client: github.MakeRetryable(httpClient, logE),
	}
}

type httpDownloader struct {
	client *http.Client
}

func (dl *httpDownloader) Download(ctx context.Context, u string) (io.ReadCloser, int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("create a http request: %w", err)
	}
	resp, err := dl.client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("send http request: %w", err)
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return resp.Body, 0, logerr.WithFields(errInvalidHTTPStatusCode, logrus.Fields{ //nolint:wrapcheck
			"http_status_code": resp.StatusCode,
		})
	}
	return resp.Body, resp.ContentLength, nil
}
