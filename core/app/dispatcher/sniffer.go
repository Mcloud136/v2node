package dispatcher

import (
	"context"

	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/protocol/bittorrent"
	"github.com/xtls/xray-core/common/protocol/http"
	"github.com/xtls/xray-core/common/protocol/quic"
	"github.com/xtls/xray-core/common/protocol/tls"
)

type SniffResult interface {
	Protocol() string
	Domain() string
}

type protocolSniffer func(context.Context, []byte) (SniffResult, error)

type protocolSnifferWithMetadata struct {
	protocolSniffer protocolSniffer
	// A Metadata sniffer will be invoked on connection establishment only, with nil body,
	// for both TCP and UDP connections
	// It will not be shown as a traffic type for routing unless there is no other successful sniffing.
	metadataSniffer bool
	network         net.Network
}

type Sniffer struct {
	sniffer []protocolSnifferWithMetadata
}

// 静态嗅探器缓存 - 这些闭包不依赖 ctx，每连接复用避免堆分配
var baseSniffers = []protocolSnifferWithMetadata{
	{func(c context.Context, b []byte) (SniffResult, error) { return http.SniffHTTP(b, c) }, false, net.Network_TCP},
	{func(c context.Context, b []byte) (SniffResult, error) { return tls.SniffTLS(b) }, false, net.Network_TCP},
	{func(c context.Context, b []byte) (SniffResult, error) { return bittorrent.SniffBittorrent(b) }, false, net.Network_TCP},
	{func(c context.Context, b []byte) (SniffResult, error) { return quic.SniffQUIC(b) }, false, net.Network_UDP},
	{func(c context.Context, b []byte) (SniffResult, error) { return bittorrent.SniffUTP(b) }, false, net.Network_UDP},
}

// defaultSniffer is a process-wide singleton for connections that do not use
// FakeDNS.  It is safe for concurrent use because Sniffer.Sniff never mutates
// the sniffer slice.
var defaultSniffer = &Sniffer{sniffer: baseSniffers}

func NewSniffer(ctx context.Context) *Sniffer {
	// When FakeDNS is not configured, return the shared singleton to avoid
	// per-connection allocation.
	sniffer, err := newFakeDNSSniffer(ctx)
	if err != nil {
		return defaultSniffer
	}
	// FakeDNS is active – build a per-request Sniffer as before.
	ret := &Sniffer{sniffer: make([]protocolSnifferWithMetadata, len(baseSniffers), len(baseSniffers)+2)}
	copy(ret.sniffer, baseSniffers)
	ret.sniffer = append(ret.sniffer, sniffer)
	fakeDNSThenOthers, err := newFakeDNSThenOthers(ctx, sniffer, baseSniffers)
	if err == nil {
		ret.sniffer = append([]protocolSnifferWithMetadata{fakeDNSThenOthers}, ret.sniffer...)
	}
	return ret
}

var errUnknownContent = errors.New("unknown content")

func (s *Sniffer) Sniff(c context.Context, payload []byte, network net.Network) (SniffResult, error) {
	var pendingSniffer []protocolSnifferWithMetadata
	for _, si := range s.sniffer {
		s := si.protocolSniffer
		if si.metadataSniffer || si.network != network {
			continue
		}
		result, err := s(c, payload)
		if err == common.ErrNoClue {
			pendingSniffer = append(pendingSniffer, si)
			continue
		}

		if err == nil && result != nil {
			return result, nil
		}
	}

	if len(pendingSniffer) > 0 {
		// 不修改 s.sniffer，避免并发竞态
		return nil, common.ErrNoClue
	}

	return nil, errUnknownContent
}

func (s *Sniffer) SniffMetadata(c context.Context) (SniffResult, error) {
	var pendingSniffer []protocolSnifferWithMetadata
	for _, si := range s.sniffer {
		s := si.protocolSniffer
		if !si.metadataSniffer {
			pendingSniffer = append(pendingSniffer, si)
			continue
		}
		result, err := s(c, nil)
		if err == common.ErrNoClue {
			pendingSniffer = append(pendingSniffer, si)
			continue
		}

		if err == nil && result != nil {
			return result, nil
		}
	}

	if len(pendingSniffer) > 0 {
		// 不修改 s.sniffer，避免并发竞态
		return nil, common.ErrNoClue
	}

	return nil, errUnknownContent
}

func CompositeResult(domainResult SniffResult, protocolResult SniffResult) SniffResult {
	return &compositeResult{domainResult: domainResult, protocolResult: protocolResult}
}

type compositeResult struct {
	domainResult   SniffResult
	protocolResult SniffResult
}

func (c compositeResult) Protocol() string {
	return c.protocolResult.Protocol()
}

func (c compositeResult) Domain() string {
	return c.domainResult.Domain()
}

func (c compositeResult) ProtocolForDomainResult() string {
	return c.domainResult.Protocol()
}

type SnifferResultComposite interface {
	ProtocolForDomainResult() string
}

type SnifferIsProtoSubsetOf interface {
	IsProtoSubsetOf(protocolName string) bool
}
