package outbound

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/Dreamacro/clash/common/convert"
	"github.com/Dreamacro/clash/component/dialer"
	C "github.com/Dreamacro/clash/constant"
	"github.com/Dreamacro/clash/transport/shadowsocks/core"
	"github.com/Dreamacro/clash/transport/shadowsocks/shadowaead"
	"github.com/Dreamacro/clash/transport/shadowsocks/shadowstream"
	"github.com/Dreamacro/clash/transport/ssr/obfs"
	"github.com/Dreamacro/clash/transport/ssr/protocol"
)

var _ C.ProxyAdapter = (*ShadowSocksR)(nil)

type ShadowSocksR struct {
	*Base
	cipher   core.Cipher
	obfs     obfs.Obfs
	protocol protocol.Protocol
}

type ShadowSocksROption struct {
	BasicOption
	Name             string `proxy:"name"`
	Server           string `proxy:"server"`
	Port             int    `proxy:"port"`
	Password         string `proxy:"password"`
	Cipher           string `proxy:"cipher"`
	Obfs             string `proxy:"obfs"`
	ObfsParam        string `proxy:"obfs-param,omitempty"`
	Protocol         string `proxy:"protocol"`
	ProtocolParam    string `proxy:"protocol-param,omitempty"`
	UDP              bool   `proxy:"udp,omitempty"`
	RandomHost       bool   `proxy:"rand-host,omitempty"`
	RemoteDnsResolve bool   `proxy:"remote-dns-resolve,omitempty"`
}

// StreamConn implements C.ProxyAdapter
func (ssr *ShadowSocksR) StreamConn(c net.Conn, metadata *C.Metadata) (net.Conn, error) {
	c = ssr.obfs.StreamConn(c)
	c = ssr.cipher.StreamConn(c)
	var (
		iv  []byte
		err error
	)
	switch conn := c.(type) {
	case *shadowstream.Conn:
		iv, err = conn.ObtainWriteIV()
		if err != nil {
			return nil, err
		}
	case *shadowaead.Conn:
		return nil, fmt.Errorf("invalid connection type")
	}
	c = ssr.protocol.StreamConn(c, iv)
	_, err = c.Write(serializesSocksAddr(metadata))
	return c, err
}

// StreamPacketConn implements C.ProxyAdapter
func (ssr *ShadowSocksR) StreamPacketConn(c net.Conn, _ *C.Metadata) (net.Conn, error) {
	if !IsPacketConn(c) {
		return c, fmt.Errorf("%s connect error: can not convert net.Conn to net.PacketConn", ssr.addr)
	}

	addr, err := resolveUDPAddr("udp", ssr.addr)
	if err != nil {
		return c, err
	}

	pc := ssr.cipher.PacketConn(c.(net.PacketConn))
	pc = ssr.protocol.PacketConn(pc)
	return WrapConn(&ssPacketConn{PacketConn: pc, rAddr: addr}), nil
}

// DialContext implements C.ProxyAdapter
func (ssr *ShadowSocksR) DialContext(ctx context.Context, metadata *C.Metadata, opts ...dialer.Option) (_ C.Conn, err error) {
	c, err := dialer.DialContext(ctx, "tcp", ssr.addr, ssr.Base.DialOptions(opts...)...)
	if err != nil {
		return nil, fmt.Errorf("%s connect error: %w", ssr.addr, err)
	}
	tcpKeepAlive(c)

	defer func(cc net.Conn, e error) {
		safeConnClose(cc, e)
	}(c, err)

	c, err = ssr.StreamConn(c, metadata)
	return NewConn(c, ssr), err
}

// ListenPacketContext implements C.ProxyAdapter
func (ssr *ShadowSocksR) ListenPacketContext(ctx context.Context, metadata *C.Metadata, opts ...dialer.Option) (C.PacketConn, error) {
	pc, err := dialer.ListenPacket(ctx, "udp", "", ssr.Base.DialOptions(opts...)...)
	if err != nil {
		return nil, err
	}

	c, err := ssr.StreamPacketConn(WrapConn(pc), metadata)
	if err != nil {
		_ = pc.Close()
		return nil, err
	}

	return NewPacketConn(c.(net.PacketConn), ssr), nil
}

func NewShadowSocksR(option ShadowSocksROption) (*ShadowSocksR, error) {
	// SSR protocol compatibility
	// https://github.com/Dreamacro/clash/pull/2056
	if strings.EqualFold(option.Cipher, "none") {
		option.Cipher = "dummy"
	}

	addr := net.JoinHostPort(option.Server, strconv.Itoa(option.Port))
	cipher := option.Cipher
	password := option.Password
	coreCiph, err := core.PickCipher(cipher, nil, password)
	if err != nil {
		return nil, fmt.Errorf("ssr %s initialize error: %w", addr, err)
	}
	var (
		ivSize int
		key    []byte
	)

	if strings.EqualFold(option.Cipher, "dummy") {
		ivSize = 0
		key = core.Kdf(option.Password, 16)
	} else {
		ciph, ok := coreCiph.(*core.StreamCipher)
		if !ok {
			return nil, fmt.Errorf("%s is not none or a supported stream cipher in ssr", cipher)
		}
		ivSize = ciph.IVSize()
		key = ciph.Key
	}

	option.Obfs = strings.ToLower(option.Obfs)
	if strings.HasPrefix(option.Obfs, "http_") && (option.RandomHost || len(option.ObfsParam) == 0) {
		option.ObfsParam = convert.RandHost()
	}

	obfsM, obfsOverhead, err := obfs.PickObfs(option.Obfs, &obfs.Base{
		Host:   option.Server,
		Port:   option.Port,
		Key:    key,
		IVSize: ivSize,
		Param:  option.ObfsParam,
	})
	if err != nil {
		return nil, fmt.Errorf("ssr %s initialize obfs error: %w", addr, err)
	}

	option.Protocol = strings.ToLower(option.Protocol)
	protocolM, err := protocol.PickProtocol(option.Protocol, &protocol.Base{
		Key:      key,
		Overhead: obfsOverhead,
		Param:    option.ProtocolParam,
	})
	if err != nil {
		return nil, fmt.Errorf("ssr %s initialize protocol error: %w", addr, err)
	}

	return &ShadowSocksR{
		Base: &Base{
			name:  option.Name,
			addr:  addr,
			tp:    C.ShadowsocksR,
			udp:   option.UDP,
			iface: option.Interface,
			rmark: option.RoutingMark,
			dns:   option.RemoteDnsResolve,
		},
		cipher:   coreCiph,
		obfs:     obfsM,
		protocol: protocolM,
	}, nil
}
