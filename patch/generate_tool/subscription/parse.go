package subscription

import (
	"errors"
	"fmt"
	"strings"

	P "github.com/sagernet/sing-box/experimental/generate_tool/subscription/protocol"
)

func isSS(url string) bool {
	return strings.HasPrefix(url, "ss://")
}

func isSSR(url string) bool {
	return strings.HasPrefix(url, "ssr://")
}

func isVMess(url string) bool {
	return strings.HasPrefix(url, "vmess://")
}

func isTrojan(url string) bool {
	return strings.HasPrefix(url, "trojan://")
}

func parse(url string) (data P.Subscription, err error) {
	if isSS(url) {
		data, err = P.ParseSSURL(url)
		if err != nil {
			err = fmt.Errorf("ss corrupted %s %s", url, err.Error())
		}
	} else if isSSR(url) {
		err = errors.New("ssr unsupported")
	} else if isVMess(url) {
		data, err = P.ParseVmessURL(url)
		if err != nil {
			err = fmt.Errorf("vmess corrupted %s %s", url, err.Error())
		}
	} else if isTrojan(url) {
		data, err = P.ParseTrojanURL(url)
		if err != nil {
			err = fmt.Errorf("trojan corrupted %s %s", url, err.Error())
		}
	} else {
		err = fmt.Errorf("unsupport %s", url)
	}
	return
}
