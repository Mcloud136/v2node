package core

import (
	"encoding/json"
	"fmt"

	"github.com/xtls/xray-core/core"
	"github.com/xtls/xray-core/infra/conf"
)

// buildDefaultOutbound builds default freedom outbound
func buildDefaultOutbound() (*core.OutboundHandlerConfig, error) {
	outboundDetourConfig := &conf.OutboundDetourConfig{
		Protocol: "freedom",
		Tag:      "Default",
	}
	proxySetting := &conf.FreedomConfig{
		DomainStrategy: "UseIPv4v6",
	}
	settingBytes, err := json.Marshal(proxySetting)
	if err != nil {
		return nil, fmt.Errorf("marshal proxy config error: %w", err)
	}
	setting := json.RawMessage(settingBytes)
	outboundDetourConfig.Settings = &setting
	return outboundDetourConfig.Build()
}

// buildBlockOutbound builds block outbound
func buildBlockOutbound() (*core.OutboundHandlerConfig, error) {
	outboundDetourConfig := &conf.OutboundDetourConfig{
		Protocol: "blackhole",
		Tag:      "block",
	}
	return outboundDetourConfig.Build()
}

// buildDnsOutbound builds dns outbound
func buildDnsOutbound() (*core.OutboundHandlerConfig, error) {
	outboundDetourConfig := &conf.OutboundDetourConfig{
		Protocol: "dns",
		Tag:      "dns_out",
	}
	return outboundDetourConfig.Build()
}
