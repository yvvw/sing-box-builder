//https://github.com/v2rayA/v2rayA/blob/main/service/core/serverObj/shadowsocks.go

package protocol

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

type Shadowsocks struct {
	Name     string `json:"name"`
	Server   string `json:"server"`
	Port     int    `json:"port"`
	Password string `json:"password"`
	Cipher   string `json:"cipher"`
	Plugin   Sip003 `json:"plugin"`
	Protocol string `json:"protocol"`
}

func (ss *Shadowsocks) GetRemark() string {
	return ss.Name
}

func (ss *Shadowsocks) GetServer() string {
	return ss.Server
}

func (ss *Shadowsocks) ToOutbound(vpnName string) (config string, err error) {
	tag := fmt.Sprintf("%s-%s", vpnName, ss.Name)
	if ss.Plugin.Name == "simple-obfs" {
		config = fmt.Sprintf(`{ "tag": "%s", "type": "shadowsocks", "server": "%s", "server_port": %d, "method": "%s", "password": "%s", "plugin": "obfs-local", "plugin_opts": "obfs=%s;obfs-host=%s" }`,
			tag, ss.Server, ss.Port, ss.Cipher, ss.Password, ss.Plugin.Opts.Obfs, ss.Plugin.Opts.Host)
	} else {
		config = fmt.Sprintf(`{ "tag": "%s", "type": "shadowsocks", "server": "%s", "server_port": %d, "method": "%s", "password": "%s" }`,
			tag, ss.Server, ss.Port, ss.Cipher, ss.Password)
	}
	return
}

func ParseSSURL(u string) (data *Shadowsocks, err error) {
	// parse attempts to parse ss:// links
	parse := func(content string) (v *Shadowsocks, ok bool) {
		// try to parse in the format of ss://BASE64(method:password)@server:port/?plugin=xxxx#name
		u, err := url.Parse(content)
		if err != nil {
			return nil, false
		}
		username := u.User.String()
		username, _ = base64URLDecode(username)
		arr := strings.SplitN(username, ":", 2)
		if len(arr) != 2 {
			return nil, false
		}
		cipher := arr[0]
		password := arr[1]
		var sip003 Sip003
		plugin := u.Query().Get("plugin")
		if len(plugin) > 0 {
			sip003 = parseSip003(plugin)
		}
		port, err := strconv.Atoi(u.Port())
		if err != nil {
			return nil, false
		}
		return &Shadowsocks{
			Cipher:   strings.ToLower(cipher),
			Password: password,
			Server:   u.Hostname(),
			Port:     port,
			Name:     strings.TrimSpace(u.Fragment),
			Plugin:   sip003,
			Protocol: "shadowsocks",
		}, true
	}
	var (
		v  *Shadowsocks
		ok bool
	)
	content := u
	// try to parse the ss:// link, if it fails, base64 decode first
	if v, ok = parse(content); !ok {
		// 进行base64解码，并unmarshal到VmessInfo上
		t := content[5:]
		var l, r string
		if ind := strings.Index(t, "#"); ind > -1 {
			l = t[:ind]
			r = t[ind+1:]
		} else {
			l = t
		}
		l, err = base64StdDecode(l)
		if err != nil {
			l, err = base64URLDecode(l)
			if err != nil {
				return
			}
		}
		t = "ss://" + l
		if len(r) > 0 {
			t += "#" + r
		}
		v, ok = parse(t)
	}
	if !ok {
		return nil, fmt.Errorf("unrecognized ss address")
	}
	return v, nil
}

type Sip003 struct {
	Name string     `json:"name"`
	Opts Sip003Opts `json:"opts"`
}

type Sip003Opts struct {
	Tls  string `json:"tls"`
	Obfs string `json:"obfs"`
	Host string `json:"host"`
	Path string `json:"uri"`
	Impl string `json:"impl"`
}

func parseSip003Opts(opts string) Sip003Opts {
	var sip003Opts Sip003Opts
	fields := strings.Split(opts, ";")
	for i := range fields {
		a := strings.Split(fields[i], "=")
		if len(a) == 1 {
			// to avoid panic
			a = append(a, "")
		}
		switch a[0] {
		case "tls":
			sip003Opts.Tls = "tls"
		case "obfs", "mode":
			sip003Opts.Obfs = a[1]
		case "obfs-path", "obfs-uri", "path":
			if !strings.HasPrefix(a[1], "/") {
				a[1] += "/"
			}
			sip003Opts.Path = a[1]
		case "obfs-host", "host":
			sip003Opts.Host = a[1]
		case "impl":
			sip003Opts.Impl = a[1]
		}
	}
	return sip003Opts
}

func parseSip003(plugin string) Sip003 {
	var sip003 Sip003
	fields := strings.SplitN(plugin, ";", 2)
	switch fields[0] {
	case "obfs-local", "simpleobfs":
		sip003.Name = "simple-obfs"
	default:
		sip003.Name = fields[0]
	}
	sip003.Opts = parseSip003Opts(fields[1])
	return sip003
}

func (s *Sip003) String() string {
	list := []string{s.Name}
	switch s.Name {
	case "simple-obfs":
		if s.Opts.Obfs != "" {
			list = append(list, "obfs="+s.Opts.Obfs)
		}
		if s.Opts.Host != "" {
			list = append(list, "obfs-host="+s.Opts.Host)
		}
		if s.Opts.Path != "" {
			list = append(list, "obfs-uri="+s.Opts.Path)
		}
		if s.Opts.Impl != "" {
			list = append(list, "impl="+s.Opts.Impl)
		}
	case "v2ray-plugin":
		if s.Opts.Tls != "" {
			list = append(list, "tls")
		}
		if s.Opts.Obfs != "" {
			list = append(list, "mode="+s.Opts.Obfs)
		}
		if s.Opts.Host != "" {
			list = append(list, "host="+s.Opts.Host)
		}
		if s.Opts.Path != "" {
			list = append(list, "path="+s.Opts.Path)
		}
		if s.Opts.Impl != "" {
			list = append(list, "impl="+s.Opts.Impl)
		}
	}
	return strings.Join(list, ";")
}
