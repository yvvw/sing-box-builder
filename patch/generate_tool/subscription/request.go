package subscription

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"

	P "github.com/sagernet/sing-box/experimental/generate_tool/subscription/protocol"
)

func Get(ctx context.Context, urlOrContent string) (subscriptions []P.Subscription, err error) {
	results, err := GetRaw(ctx, urlOrContent)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("empty subscription url or content %s", urlOrContent)
	}

	for _, raw := range results {
		subscription, err := parse(raw)
		if err != nil {
			fmt.Printf("%s parse failed, pass\n", raw)
			continue
		}
		subscriptions = append(subscriptions, subscription)
	}
	if len(subscriptions) == 0 {
		return nil, fmt.Errorf("empty subscription url or content %s", urlOrContent)
	}
	return
}

func GetRaw(ctx context.Context, urlOrContent string) ([]string, error) {
	var rawBytes []byte
	var err error
	if strings.HasPrefix(urlOrContent, "http") {
		rawBytes, err = httpGetBase64(ctx, urlOrContent)
		if err != nil {
			return nil, err
		}
	} else {
		rawBytes, err = base64.StdEncoding.DecodeString(urlOrContent)
		if err != nil {
			return nil, err
		}
	}
	return strings.Split(strings.TrimSpace(string(rawBytes)), "\n"), nil
}

func httpGetBase64(ctx context.Context, url string) ([]byte, error) {
	bodyBytes, err := httpGet(ctx, url)
	if err != nil {
		return nil, err
	}
	bodyBytes, err = base64.StdEncoding.DecodeString(string(bodyBytes))
	if err != nil {
		return nil, err
	}
	return bodyBytes, nil
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
