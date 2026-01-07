package subscription

import (
	"context"
	"io"
	"net/http"
	"strings"

	"github.com/sagernet/sing-box/experimental/tools_generate/subscription/parser"
	"github.com/sagernet/sing-box/option"
)

func Get(ctx context.Context, urlOrContent string) (subscriptions []option.Outbound, err error) {
	if strings.HasPrefix(urlOrContent, "http") {
		contentBytes, err := httpGet(ctx, urlOrContent)
		if err != nil {
			return nil, err
		}
		return parser.ParseSubscription(ctx, string(contentBytes))
	}

	return parser.ParseSubscription(ctx, urlOrContent)
}

func httpGet(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129.0.0.0 Safari/537.36")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	return io.ReadAll(resp.Body)
}
