package tools_generate

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"reflect"
	"slices"
	"sort"
	"sync"
	"text/template"

	"github.com/BurntSushi/toml"
	"github.com/sagernet/sing-box/option"

	S "github.com/sagernet/sing-box/experimental/tools_generate/subscription"
	U "github.com/sagernet/sing-box/experimental/tools_generate/utils"
	"github.com/sagernet/sing/common/json"
)

type Config struct {
	SubscriptionList []subscriptionConfig `toml:"subscriptions"`
	SingBox          singBoxConfig        `toml:"sing-box"`
}

func Parse(configBytes []byte) (*Config, error) {
	config := &Config{}
	_, err := toml.Decode(string(configBytes), config)
	if err != nil {
		return nil, err
	}
	return config, nil
}

type subscriptionConfig struct {
	Name            string `toml:"name"`
	URL             string `toml:"url"`
	Content         string `toml:"content"`
	DefaultOutbound string `toml:"default"`
}

type singBoxConfig struct {
	Template         string   `toml:"template"`
	Output           string   `toml:"output"`
	Gateway          string   `toml:"gateway"`
	ClashPort        int      `toml:"clash_port"`
	DefaultOutbound  string   `toml:"default"`
	AutoOutboundList []string `toml:"auto_outbounds"`
	IncludeServer    bool     `toml:"include_server"`

	RuleSetList   []singBoxRuleSetConfig       `toml:"rule_set"`
	DirectRule    singBoxRouteRuleDirectConfig `toml:"direct_rule"`
	ProxyRule     singBoxRouteRuleProxyConfig  `toml:"proxy_rule"`
	BlockRule     singBoxRouteRuleBlockConfig  `toml:"block_rule"`
	DNSRuleList   []singBoxDNSRuleConfig       `toml:"dns_rules"`
	RouteRuleList []singBoxRouteRuleConfig     `toml:"route_rules"`
}

type singBoxRuleSetConfig struct {
	Tag            string `toml:"tag"`
	Url            string `toml:"url"`
	DownloadDetour string `toml:"download_detour"`
}

type singBoxDNSRuleConfig struct {
	Server       string   `toml:"server"`
	Domain       []string `toml:"domain"`
	DomainSuffix []string `toml:"domain_suffix"`
	RuleSet      []string `toml:"rule_set"`
}

type singBoxRouteRuleBaseConfig struct {
	Domain       []string `toml:"domain"`
	DomainSuffix []string `toml:"domain_suffix"`
	RuleSet      []string `toml:"rule_set"`
}

type singBoxRouteRuleDirectConfig struct {
	singBoxRouteRuleBaseConfig
	IPCIDR []string `toml:"ip_cidr"`
}

type singBoxRouteRuleProxyConfig struct {
	singBoxRouteRuleDirectConfig
}

type singBoxRouteRuleBlockConfig struct {
	singBoxRouteRuleBaseConfig
}

type singBoxRouteRuleConfig struct {
	singBoxRouteRuleProxyConfig
	Action   string `toml:"action" default:"route"`
	Outbound string `toml:"outbound"`
}

func GenerateSingBoxConfig(config *Config) ([]byte, error) {
	tmplBytes, err := os.ReadFile(config.SingBox.Template)
	if err != nil {
		return nil, err
	}

	tmpl, err := template.New("sing-box").
		Funcs(template.FuncMap{
			"MarshalArray":  U.MarshalArrayF,
			"ConcatStrings": U.Concat[string],
		}).
		Parse(string(tmplBytes))
	if err != nil {
		return nil, err
	}

	ctx := context.Background()

	subscriptionList, err := getSubscriptions(ctx, config)
	if err != nil {
		return nil, err
	}

	var outbounds []string
	var outboundTags []string
	var outboundDomains []string
	var outboundGroups []map[string]any
	var outboundGroupTags []string

	for idx, subCfg := range config.SubscriptionList {
		var subOutboundTags []string
		subscriptions := subscriptionList[idx]
		for _, subscription := range subscriptions {
			subscription.Tag = subCfg.Name + "-" + subscription.Tag
			outbound, err := subscription.MarshalJSONContext(ctx)
			if err != nil {
				return nil, err
			}
			outbounds = append(outbounds, string(outbound))
			subOutboundTags = append(subOutboundTags, subscription.Tag)
			if config.SingBox.IncludeServer {
				var options struct {
					Server string `json:"server"`
				}
				err := json.Unmarshal(outbound, &options)
				if err != nil {
					continue
				}
				outboundDomains = append(outboundDomains, options.Server)
			}
		}
		sort.Strings(subOutboundTags)
		outboundTags = append(outboundTags, subOutboundTags...)

		subscriptionOutboundGroupTag := "out-" + subCfg.Name
		outboundGroupTags = append(outboundGroupTags, subscriptionOutboundGroupTag)

		defaultSubscriptionOutboundTag := subCfg.Name + "-" + subCfg.DefaultOutbound
		if !slices.Contains(subOutboundTags, defaultSubscriptionOutboundTag) {
			defaultSubscriptionOutboundTag = subOutboundTags[0]
		}

		outboundGroups = append(outboundGroups, map[string]any{
			"Tag":                subscriptionOutboundGroupTag,
			"DefaultOutboundTag": defaultSubscriptionOutboundTag,
			"OutboundTags":       subOutboundTags,
		})
	}

	var autoOutbounds []string
	{
		for _, tag := range config.SingBox.AutoOutboundList {
			if U.Contains(outboundTags, tag) {
				autoOutbounds = append(autoOutbounds, tag)
			}
		}
		autoOutbounds = U.Unique(autoOutbounds)
		if len(autoOutbounds) > 1 {
			outboundGroupTags = append([]string{"out-proxy-auto"}, outboundGroupTags...)
		}
	}

	tmplBuffer := &bytes.Buffer{}
	err = tmpl.Execute(tmplBuffer, struct {
		DNSRules   []string
		RouteRules []string
		RuleSet    []string

		Gateway   string
		ClashPort int

		OutboundTags       []string
		DefaultOutboundTag string
		OutboundGroups     []map[string]any
		Outbounds          []string
		AutoOutbounds      []string

		DirectDomains []string
		ProxyDomains  []string
		BlockDomains  []string

		DirectDomainSuffixes []string
		ProxyDomainSuffixes  []string
		BlockDomainSuffixes  []string

		DirectIPs []string
		ProxyIPs  []string

		DirectRuleSet []string
		ProxyRuleSet  []string
		BlockRuleSet  []string
	}{
		DNSRules:   marshal(config.SingBox.DNSRuleList),
		RouteRules: marshal(config.SingBox.RouteRuleList),
		RuleSet:    marshal(config.SingBox.RuleSetList),

		Gateway:   config.SingBox.Gateway,
		ClashPort: config.SingBox.ClashPort,

		OutboundTags: outboundGroupTags,
		DefaultOutboundTag: func() string {
			if !slices.Contains(outboundGroupTags, config.SingBox.DefaultOutbound) {
				return outboundGroupTags[0]
			}
			return config.SingBox.DefaultOutbound
		}(),
		OutboundGroups: outboundGroups,
		Outbounds:      outbounds,
		AutoOutbounds:  autoOutbounds,

		DirectDomains: func() []string {
			var subscriptionDomains []string
			for _, subscription := range config.SubscriptionList {
				if subscription.URL == "" {
					continue
				}
				u, err := url.Parse(subscription.URL)
				if err == nil {
					subscriptionDomains = append(subscriptionDomains, u.Hostname())
				}
			}
			return U.Unique(config.SingBox.DirectRule.Domain, subscriptionDomains, outboundDomains)
		}(),
		ProxyDomains: config.SingBox.ProxyRule.Domain,
		BlockDomains: config.SingBox.BlockRule.Domain,

		DirectDomainSuffixes: config.SingBox.DirectRule.DomainSuffix,
		ProxyDomainSuffixes:  config.SingBox.ProxyRule.DomainSuffix,
		BlockDomainSuffixes:  config.SingBox.BlockRule.DomainSuffix,

		DirectIPs: config.SingBox.DirectRule.IPCIDR,
		ProxyIPs:  config.SingBox.ProxyRule.IPCIDR,

		DirectRuleSet: config.SingBox.DirectRule.RuleSet,
		ProxyRuleSet:  config.SingBox.ProxyRule.RuleSet,
		BlockRuleSet:  config.SingBox.BlockRule.RuleSet,
	})
	if err != nil {
		return nil, err
	}

	return tmplBuffer.Bytes(), nil
}

func getSubscriptions(ctx context.Context, config *Config) (outboundList [][]option.Outbound, err error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	outboundList = make([][]option.Outbound, len(config.SubscriptionList))
	wg := sync.WaitGroup{}
	wg.Add(len(config.SubscriptionList))
	for idx, subConfig := range config.SubscriptionList {
		go func() {
			defer wg.Done()

			var outbounds []option.Outbound
			var sErr error
			if subConfig.URL != "" {
				outbounds, sErr = S.Get(ctx, subConfig.URL)
			} else if subConfig.Content != "" {
				outbounds, sErr = S.Get(ctx, subConfig.Content)
			} else {
				sErr = errors.New("empty url and content")
			}
			if sErr == nil && len(outbounds) == 0 {
				sErr = fmt.Errorf("empty outbounds %v", subConfig)
			}
			if sErr == nil {
				outboundList[idx] = outbounds
				return
			}
			if sErr != nil && !errors.Is(sErr, context.Canceled) {
				err = sErr
				cancel()
			}
		}()
	}
	wg.Wait()
	return
}

func marshal(list any) (results []string) {
	value := reflect.ValueOf(list)
	length := value.Len()
	for i := 0; i < length; i++ {
		bs, err := json.Marshal(value.Index(i).Interface())
		if err != nil {
			continue
		}
		results = append(results, string(bs))
	}
	return
}

func (cfg singBoxDNSRuleConfig) MarshalJSON() ([]byte, error) {
	bs := make([]byte, 0)
	bs = append(bs, []byte(fmt.Sprintf(`{ "server": "%s"`, cfg.Server))...)
	if len(cfg.RuleSet) > 0 {
		bs = append(bs, []byte(fmt.Sprintf(`, "rule_set": %s`, U.MarshalArrayF(cfg.RuleSet)))...)
	}
	if len(cfg.Domain) > 0 {
		bs = append(bs, []byte(fmt.Sprintf(`, "domain": %s`, U.MarshalArrayF(cfg.Domain)))...)
	}
	if len(cfg.DomainSuffix) > 0 {
		bs = append(bs, []byte(fmt.Sprintf(`, "domain_suffix": %s`, U.MarshalArrayF(cfg.DomainSuffix)))...)
	}
	bs = append(bs, []byte(" }")...)
	return bs, nil
}

func (cfg singBoxRuleSetConfig) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`{ "tag": "%s", "url": "%s", "type": "remote", "format": "binary", "download_detour": "%s" }`, cfg.Tag, cfg.Url, cfg.DownloadDetour)), nil
}

func (cfg singBoxRouteRuleConfig) MarshalJSON() ([]byte, error) {
	bs := make([]byte, 0)
	action := cfg.Action
	if action == "" {
		action = "route"
	}
	bs = append(bs, []byte(fmt.Sprintf(`{ "action": "%s", "outbound": "%s"`, action, cfg.Outbound))...)
	if len(cfg.RuleSet) > 0 {
		bs = append(bs, []byte(fmt.Sprintf(`, "rule_set": %s`, U.MarshalArrayF(cfg.RuleSet)))...)
	}
	if len(cfg.Domain) > 0 {
		bs = append(bs, []byte(fmt.Sprintf(`, "domain": %s`, U.MarshalArrayF(cfg.Domain)))...)
	}
	if len(cfg.DomainSuffix) > 0 {
		bs = append(bs, []byte(fmt.Sprintf(`, "domain_suffix": %s`, U.MarshalArrayF(cfg.DomainSuffix)))...)
	}
	if len(cfg.IPCIDR) > 0 {
		bs = append(bs, []byte(fmt.Sprintf(`, "ip_cidr": %s`, U.MarshalArrayF(cfg.IPCIDR)))...)
	}
	bs = append(bs, []byte(" }")...)
	return bs, nil
}
