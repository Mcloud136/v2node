package core

import (
	"encoding/json"
	"net"
	"sync"

	panel "github.com/wyx2685/v2node/api/v2board"
	"github.com/xtls/xray-core/app/dns"
	"github.com/xtls/xray-core/app/router"
	xnet "github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/core"
	coreConf "github.com/xtls/xray-core/infra/conf"
)

var (
	publicIPv6CheckOnce sync.Once
	publicIPv6Result    bool
)

// hasPublicIPv6 checks if the machine has a public IPv6 address (cached)
func hasPublicIPv6() bool {
	publicIPv6CheckOnce.Do(func() {
		addrs, err := net.InterfaceAddrs()
		if err != nil {
			return
		}
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			ip := ipNet.IP
			// Check if it's IPv6, not loopback, not link-local, not private/ULA
			if ip.To4() == nil && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() && !ip.IsPrivate() {
				publicIPv6Result = true
				break
			}
		}
	})
	return publicIPv6Result
}

func hasOutboundWithTag(list []*core.OutboundHandlerConfig, tag string) bool {
	for _, o := range list {
		if o != nil && o.Tag == tag {
			return true
		}
	}
	return false
}

func buildAndAddRule(info *panel.NodeInfo, route panel.Route, outboundTag string, matchKey string) json.RawMessage {
	var rule map[string]interface{}
	if matchKey == "port" {
		rule = map[string]interface{}{
			"inboundTag":  info.Tag,
			"port":        route.Match,
			"outboundTag": outboundTag,
		}
	} else if matchKey == "network" {
		rule = map[string]interface{}{
			"inboundTag":  info.Tag,
			"network":     route.Match,
			"outboundTag": outboundTag,
		}
	} else {
		rule = map[string]interface{}{
			"inboundTag":  info.Tag,
			matchKey:      route.Match,
			"outboundTag": outboundTag,
		}
	}
	rawRule, _ := json.Marshal(rule)
	return rawRule
}

func processOutboundRoute(info *panel.NodeInfo, route panel.Route, matchKey string, coreRouterConfig *coreConf.RouterConfig, coreOutboundConfig *[]*core.OutboundHandlerConfig) {
	if route.ActionValue == nil {
		return
	}
	outbound := &coreConf.OutboundDetourConfig{}
	if err := json.Unmarshal([]byte(*route.ActionValue), outbound); err != nil {
		return
	}
	rule := buildAndAddRule(info, route, outbound.Tag, matchKey)
	if rule != nil {
		coreRouterConfig.RuleList = append(coreRouterConfig.RuleList, rule)
	}
	if hasOutboundWithTag(*coreOutboundConfig, outbound.Tag) {
		return
	}
	if custom_outbound, err := outbound.Build(); err == nil {
		*coreOutboundConfig = append(*coreOutboundConfig, custom_outbound)
	}
}

func GetCustomConfig(infos []*panel.NodeInfo) (*dns.Config, []*core.OutboundHandlerConfig, *router.Config, error) {
	//dns
	queryStrategy := "UseIPv4v6"
	if !hasPublicIPv6() {
		queryStrategy = "UseIPv4"
	}

	// 预计算总路由数量，用于容量预分配
	totalRoutes := 0
	for _, info := range infos {
		totalRoutes += len(info.Common.Routes)
	}

	coreDnsConfig := &coreConf.DNSConfig{
		Servers: make([]*coreConf.NameServerConfig, 0, 1+totalRoutes/4),
	}
	// 添加默认 DNS 服务器
	coreDnsConfig.Servers = append(coreDnsConfig.Servers, &coreConf.NameServerConfig{
		Address: &coreConf.Address{
			Address: xnet.ParseAddress("localhost"),
		},
	})
	coreDnsConfig.QueryStrategy = queryStrategy

	//outbound
	defaultoutbound, _ := buildDefaultOutbound()
	coreOutboundConfig := make([]*core.OutboundHandlerConfig, 0, 3+totalRoutes/4)
	coreOutboundConfig = append(coreOutboundConfig, defaultoutbound)
	block, _ := buildBlockOutbound()
	coreOutboundConfig = append(coreOutboundConfig, block)
	dns, _ := buildDnsOutbound()
	coreOutboundConfig = append(coreOutboundConfig, dns)

	//route
	domainStrategy := "AsIs"
	dnsRule, _ := json.Marshal(map[string]interface{}{
		"port":        "53",
		"network":     "udp",
		"outboundTag": "dns_out",
	})
	coreRouterConfig := &coreConf.RouterConfig{
		RuleList:       make([]json.RawMessage, 0, 1+totalRoutes),
		DomainStrategy: &domainStrategy,
	}
	coreRouterConfig.RuleList = append(coreRouterConfig.RuleList, dnsRule)

	for _, info := range infos {
		if len(info.Common.Routes) == 0 {
			continue
		}
		for _, route := range info.Common.Routes {
			switch route.Action {
			case "dns":
				if route.ActionValue == nil {
					continue
				}
				server := &coreConf.NameServerConfig{
					Address: &coreConf.Address{
						Address: xnet.ParseAddress(*route.ActionValue),
					},
				}
				if len(route.Match) != 0 {
					server.Domains = route.Match
					server.SkipFallback = true
				}
				coreDnsConfig.Servers = append(coreDnsConfig.Servers, server)
			case "block":
				if rule := buildAndAddRule(info, route, "block", "domain"); rule != nil {
					coreRouterConfig.RuleList = append(coreRouterConfig.RuleList, rule)
				}
			case "block_ip":
				if rule := buildAndAddRule(info, route, "block", "ip"); rule != nil {
					coreRouterConfig.RuleList = append(coreRouterConfig.RuleList, rule)
				}
			case "block_port":
				if rule := buildAndAddRule(info, route, "block", "port"); rule != nil {
					coreRouterConfig.RuleList = append(coreRouterConfig.RuleList, rule)
				}
			case "protocol":
				if rule := buildAndAddRule(info, route, "block", "protocol"); rule != nil {
					coreRouterConfig.RuleList = append(coreRouterConfig.RuleList, rule)
				}
			case "route":
				processOutboundRoute(info, route, "domain", coreRouterConfig, &coreOutboundConfig)
			case "route_ip":
				processOutboundRoute(info, route, "ip", coreRouterConfig, &coreOutboundConfig)
			case "default_out":
				processOutboundRoute(info, route, "network", coreRouterConfig, &coreOutboundConfig)
			default:
				continue
			}
		}
	}
	DnsConfig, err := coreDnsConfig.Build()
	if err != nil {
		return nil, nil, nil, err
	}
	RouterConfig, err := coreRouterConfig.Build()
	if err != nil {
		return nil, nil, nil, err
	}
	return DnsConfig, coreOutboundConfig, RouterConfig, nil
}
