//https://github.com/v2rayA/v2rayA/blob/feat_v5/service/core/serverObj/v2ray.go#L32

package protocol

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	T "github.com/sagernet/sing-box/experimental/generate_tool/subscription/protocol/mtype"
	"github.com/sagernet/sing/common/json"
)

type V2Ray struct {
	Ps            string           `json:"ps"`
	Add           string           `json:"add"`
	Port          T.StringOrNumber `json:"port"`
	ID            string           `json:"id"`
	Aid           T.StringOrNumber `json:"aid"`
	Security      string           `json:"scy"`
	Net           string           `json:"net"`
	Type          string           `json:"type"`
	Host          string           `json:"host"`
	SNI           string           `json:"sni"`
	Path          string           `json:"path"`
	TLS           string           `json:"tls"`
	Flow          string           `json:"flow,omitempty"`
	Alpn          string           `json:"alpn,omitempty"`
	AllowInsecure bool             `json:"allowInsecure"`
	V             string           `json:"v"`
	Protocol      string           `json:"protocol"`
}

func (vr *V2Ray) GetRemark() string {
	return vr.Ps
}

func (vr *V2Ray) GetServer() string {
	return vr.Add
}

func (vr *V2Ray) ToOutbound(vpnName string) (config string, err error) {
	port, err := strconv.Atoi(string(vr.Port))
	if err != nil {
		return "", fmt.Errorf("bad port vr %s", vr.Port)
	}
	alterId, err := strconv.Atoi(string(vr.Aid))
	if err != nil {
		return "", fmt.Errorf("bad alter id vr %s", vr.Aid)
	}

	var transport string
	if vr.Net == "ws" {
		transport = fmt.Sprintf(`, "transport": { "type": "ws", "path": "%s" }`, vr.Path)
	}
	var tls string
	if vr.TLS == "tls" {
		tls = fmt.Sprintf(`, "tls": { "enabled": true }`)
	}
	if vr.Protocol == "vmess" {
		config = fmt.Sprintf(`{ "tag": "%s-%s", "type": "vmess", "server": "%s", "server_port": %d, "uuid": "%s", "alter_id": %d%s%s }`,
			vpnName, vr.Ps, vr.Add, port, vr.ID, alterId, tls, transport)
	}
	if config == "" {
		err = errors.New("v2ray convert to sing-box outbound failed")
	}
	return
}

func ParseVlessURL(vless string) (data *V2Ray, err error) {
	u, err := url.Parse(vless)
	if err != nil {
		return nil, err
	}
	data = &V2Ray{
		Ps:            u.Fragment,
		Add:           u.Hostname(),
		Port:          T.StringOrNumber(u.Port()),
		ID:            u.User.String(),
		Net:           u.Query().Get("type"),
		Type:          u.Query().Get("headerType"),
		Host:          u.Query().Get("host"),
		SNI:           u.Query().Get("sni"),
		Path:          u.Query().Get("path"),
		TLS:           u.Query().Get("security"),
		Flow:          u.Query().Get("flow"),
		Alpn:          u.Query().Get("alpn"),
		AllowInsecure: u.Query().Get("allowInsecure") == "true",
		Protocol:      "vless",
	}
	if data.Net == "" {
		data.Net = "tcp"
	}
	if data.Net == "grpc" {
		data.Path = u.Query().Get("serviceName")
	}
	if data.Type == "" {
		data.Type = "none"
	}
	if data.TLS == "" {
		data.TLS = "none"
	}
	if data.Flow == "" {
		data.Flow = "xtls-rprx-direct"
	}
	if data.Net == "mkcp" || data.Net == "kcp" {
		data.Path = u.Query().Get("seed")
	}
	return data, nil
}

func ParseVmessURL(vmess string) (data *V2Ray, err error) {
	var info V2Ray
	// not in json format, try to resolve as vmess://BASE64(Security:ID@Add:v2rayString)?remarks=Ps&obfsParam=Host&Path=Path&obfs=Net&tls=TLS
	var u *url.URL
	u, err = url.Parse(vmess)
	if err != nil {
		return
	}
	re := regexp.MustCompile(`.*:(.+)@(.+):(\d+)`)
	s := strings.Split(vmess[8:], "?")[0]
	s, err = base64StdDecode(s)
	if err != nil {
		s, err = base64URLDecode(s)
	}
	subMatch := re.FindStringSubmatch(s)
	if subMatch == nil {
		err = fmt.Errorf("unrecognized vmess address")
		return
	}
	q := u.Query()
	ps := q.Get("remarks")
	if ps == "" {
		ps = q.Get("remark")
	}
	obfs := q.Get("obfs")
	obfsParam := q.Get("obfsParam")
	path := q.Get("path")
	if obfs == "kcp" || obfs == "mkcp" {
		m := make(map[string]string)
		//cater to v2rayN definition
		_ = json.Unmarshal([]byte(obfsParam), &m)
		path = m["seed"]
		obfsParam = ""
	}
	aid := q.Get("alterId")
	if aid == "" {
		aid = q.Get("aid")
	}
	security := q.Get("scy")
	if security == "" {
		security = q.Get("security")
	}
	sni := q.Get("sni")
	info = V2Ray{
		ID:            subMatch[1],
		Add:           subMatch[2],
		Port:          T.StringOrNumber(subMatch[3]),
		Ps:            ps,
		Host:          obfsParam,
		Path:          path,
		SNI:           sni,
		Net:           obfs,
		Aid:           T.StringOrNumber(aid),
		Security:      security,
		TLS:           map[string]string{"1": "tls"}[q.Get("tls")],
		AllowInsecure: false,
	}
	if info.Net == "websocket" {
		info.Net = "ws"
	}
	// correct the wrong vmess as much as possible
	if strings.HasPrefix(info.Host, "/") && info.Path == "" {
		info.Path = info.Host
		info.Host = ""
	}
	if info.Aid == "" {
		info.Aid = "0"
	}
	info.Protocol = "vmess"
	return &info, nil
}
