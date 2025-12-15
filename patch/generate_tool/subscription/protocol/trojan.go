package protocol

import (
	"fmt"
	"net/url"
	"strconv"
)

type Trojan struct {
	Name          string `json:"name"`
	Server        string `json:"server"`
	Port          int    `json:"port"`
	Password      string `json:"password"`
	Sni           string `json:"sni"`
	Type          string `json:"type"`
	Encryption    string `json:"encryption"`
	Host          string `json:"host"`
	Path          string `json:"path"`
	AllowInsecure bool   `json:"allowInsecure"`
	Protocol      string `json:"protocol"`
}

func (tj *Trojan) GetRemark() string {
	return tj.Name
}

func (tj *Trojan) GetServer() string {
	return tj.Server
}

func (tj *Trojan) ToOutbound(vpnName string) (config string, err error) {
	config = fmt.Sprintf(`{ "tag": "%s-%s", "type": "trojan", "server": "%s", "server_port": %d, "password": "%s"`,
		vpnName, tj.Name, tj.Server, tj.Port, tj.Password)
	if tj.Sni != "" {
		config += fmt.Sprintf(`, "network": "tcp" ,"tls": { "enabled": true, "insecure": %t, "server_name": "%s" }`,
			tj.AllowInsecure, tj.Sni)
	}
	config += " }"
	return
}

func ParseTrojanURL(u string) (data *Trojan, err error) {
	//trojan://password@server:port#escape(remarks)
	t, err := url.Parse(u)
	if err != nil {
		err = fmt.Errorf("invalid trojan format")
		return
	}
	allowInsecure := t.Query().Get("allowInsecure")
	if t.Query().Get("tls") == "false" {
		allowInsecure = "true"
	}
	sni := t.Query().Get("peer")
	if sni == "" {
		sni = t.Query().Get("sni")
	}
	if sni == "" {
		sni = t.Hostname()
	}
	port, err := strconv.Atoi(t.Port())
	if err != nil {
		return nil, fmt.Errorf("invalid parameter")
	}
	data = &Trojan{
		Name:          t.Fragment,
		Server:        t.Hostname(),
		Port:          port,
		Password:      t.User.Username(),
		Sni:           sni,
		AllowInsecure: allowInsecure == "1" || allowInsecure == "true",
		Protocol:      "trojan",
	}
	if t.Scheme == "trojan-go" {
		data.Protocol = "trojan-go"
		data.Encryption = t.Query().Get("encryption")
		data.Host = t.Query().Get("host")
		data.Path = t.Query().Get("path")
		data.Type = t.Query().Get("type")
		data.AllowInsecure = false
	}
	return data, nil
}
