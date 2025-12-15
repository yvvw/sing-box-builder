package generate_tool

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

	S "github.com/sagernet/sing-box/experimental/generate_tool/subscription"
	P "github.com/sagernet/sing-box/experimental/generate_tool/subscription/protocol"
	U "github.com/sagernet/sing-box/experimental/generate_tool/utils"
	"github.com/sagernet/sing/common/json"
)

type Config struct {
	Subscriptions []subscriptionConfig `toml:"subscriptions"`
	SingBox       singBoxConfig        `toml:"sing-box"`
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
	Name    string `toml:"name"`
	URL     string `toml:"url"`
	Content string `toml:"content"`
	Default string `toml:"default"`
}

type singBoxConfig struct {
	Template      string                       `toml:"template"`
	Output        string                       `toml:"output"`
	Gateway       string                       `toml:"gateway"`
	ClashPort     int                          `toml:"clash_port"`
	Default       string                       `toml:"default"`
	AutoOutbounds []string                     `toml:"auto_outbounds"`
	IncludeServer bool                         `toml:"include_server"`
	DirectRule    singBoxRouteRuleDirectConfig `toml:"direct_rule"`
	ProxyRule     singBoxRouteRuleProxyConfig  `toml:"proxy_rule"`
	BlockRule     singBoxRouteRuleBlockConfig  `toml:"block_rule"`
	DNSRules      []singBoxDNSRuleConfig       `toml:"dns_rules"`
	RuleSet       []singBoxRuleSetConfig       `toml:"rule_set"`
	RouteRules    []singBoxRouteRuleConfig     `toml:"route_rules"`
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

type singBoxDNSRuleConfig struct {
	Server       string   `toml:"server"`
	Domain       []string `toml:"domain"`
	DomainSuffix []string `toml:"domain_suffix"`
	RuleSet      []string `toml:"rule_set"`
}

type singBoxRuleSetConfig struct {
	Tag            string `toml:"tag"`
	Url            string `toml:"url"`
	DownloadDetour string `toml:"download_detour"`
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

	tmpl, err := template.New("sing-box").Funcs(template.FuncMap{
		"MarshalArray":  U.MarshalArrayF,
		"ConcatStrings": U.Concat[string],
	}).Parse(string(tmplBytes))
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	subscriptionList := make([][]P.Subscription, len(config.Subscriptions))

	wg := sync.WaitGroup{}
	for idx, subscriptionConfig := range config.Subscriptions {
		wg.Add(1)

		go func() {
			defer wg.Done()

			var subscriptions []P.Subscription
			var sErr error
			if subscriptionConfig.URL != "" {
				subscriptions, sErr = S.Get(ctx, subscriptionConfig.URL)
			} else if subscriptionConfig.Content != "" {
				subscriptions, sErr = S.Get(ctx, subscriptionConfig.Content)
			} else {
				sErr = errors.New("empty url and content")
			}
			if sErr == nil && len(subscriptions) == 0 {
				sErr = fmt.Errorf("empty subscriptions %v", subscriptionConfig)
			}
			if sErr == nil {
				subscriptionList[idx] = subscriptions
				return
			}
			if sErr != nil && !errors.Is(sErr, context.Canceled) {
				err = sErr
				cancel()
			}
		}()
	}
	wg.Wait()

	if err != nil {
		return nil, err
	}

	var outbounds []string
	var outboundTags []string
	var outboundDomains []string
	var outboundGroups []map[string]any
	var outboundGroupTags []string

	for idx, subscriptionCfg := range config.Subscriptions {
		subscriptions := subscriptionList[idx]

		var subscriptionOutboundTags []string
		for _, subscription := range subscriptions {
			outbound, err := subscription.ToOutbound(subscriptionCfg.Name)
			if err != nil {
				return nil, err
			}
			outbounds = append(outbounds, outbound)
			subscriptionOutboundTags = append(subscriptionOutboundTags, subscriptionCfg.Name+"-"+subscription.GetRemark())
			if config.SingBox.IncludeServer {
				outboundDomains = append(outboundDomains, subscription.GetServer())
			}
		}
		sort.Strings(subscriptionOutboundTags)
		outboundTags = append(outboundTags, subscriptionOutboundTags...)

		subscriptionOutboundGroupTag := "out-" + subscriptionCfg.Name
		outboundGroupTags = append(outboundGroupTags, subscriptionOutboundGroupTag)

		defaultSubscriptionOutboundTag := subscriptionCfg.Name + "-" + subscriptionCfg.Default
		if !slices.Contains(subscriptionOutboundTags, defaultSubscriptionOutboundTag) {
			defaultSubscriptionOutboundTag = subscriptionOutboundTags[0]
		}

		outboundGroups = append(outboundGroups, map[string]any{
			"Tag":                subscriptionOutboundGroupTag,
			"DefaultOutboundTag": defaultSubscriptionOutboundTag,
			"OutboundTags":       subscriptionOutboundTags,
		})
	}

	var autoOutbounds []string
	{
		for _, tag := range config.SingBox.AutoOutbounds {
			if U.Contains(outboundTags, tag) {
				autoOutbounds = append(autoOutbounds, tag)
			}
		}
		autoOutbounds = U.Unique(autoOutbounds)
		if len(autoOutbounds) > 1 {
			outboundGroupTags = append([]string{"out-proxy-auto"}, outboundGroupTags...)
		}
	}

	templateBuffer := &bytes.Buffer{}
	err = tmpl.Execute(templateBuffer, struct {
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
		DNSRules:   marshal(config.SingBox.DNSRules),
		RouteRules: marshal(config.SingBox.RouteRules),
		RuleSet:    marshal(config.SingBox.RuleSet),

		Gateway:   config.SingBox.Gateway,
		ClashPort: config.SingBox.ClashPort,

		OutboundTags: outboundGroupTags,
		DefaultOutboundTag: func() string {
			if !slices.Contains(outboundGroupTags, config.SingBox.Default) {
				return outboundGroupTags[0]
			}
			return config.SingBox.Default
		}(),
		OutboundGroups: outboundGroups,
		Outbounds:      outbounds,
		AutoOutbounds:  autoOutbounds,

		DirectDomains: func() []string {
			var subscriptionDomains []string
			for _, subscription := range config.Subscriptions {
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

	return templateBuffer.Bytes(), nil
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
