package dns

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/netip"
	"strconv"
	"sync"
	"time"

	"github.com/lucas-clemente/quic-go"
	D "github.com/miekg/dns"
	"github.com/phuslu/log"

	"github.com/Dreamacro/clash/component/dialer"
	"github.com/Dreamacro/clash/component/resolver"
)

type doqClient struct {
	sync.RWMutex
	r            *Resolver
	port         string
	host         string
	ip           netip.Addr
	proxyAdapter string
	connection   quic.Connection
}

func (dc *doqClient) Exchange(m *D.Msg) (msg *D.Msg, err error) {
	return dc.ExchangeContext(context.Background(), m)
}

func (dc *doqClient) ExchangeContext(ctx context.Context, m *D.Msg) (msg *D.Msg, err error) {
	newM := *m
	newM.Id = 0

	bufMsg, err := (&newM).Pack()
	if err != nil {
		return nil, err
	}

	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, time.Now().Add(resolver.DefaultDNSTimeout))
		defer cancel()
	}

	conn, err := dc.openStream(ctx)
	if err != nil {
		return nil, err
	}

	bufReq := make([]byte, 2+len(bufMsg))
	binary.BigEndian.PutUint16(bufReq, uint16(len(bufMsg)))
	copy(bufReq[2:], bufMsg)

	_, err = conn.Write(bufReq)
	if err != nil {
		return nil, err
	}

	_ = conn.Close()

	var length uint16
	if err = binary.Read(conn, binary.BigEndian, &length); err != nil {
		return nil, err
	}

	buff := make([]byte, length)
	_, err = io.ReadFull(conn, buff)
	if err != nil {
		return nil, err
	}

	msg = &D.Msg{}
	err = msg.Unpack(buff)

	if err == nil {
		msg.Id = m.Id

		q := m.Question[0]
		if q.Qtype == D.TypeA || q.Qtype == D.TypeAAAA {
			var (
				ans []string
				pr  string
			)
			for _, r := range msg.Answer {
				if r != nil {
					if a, ok := r.(*D.A); ok && a.A != nil {
						ans = append(ans, a.A.String())
					} else if a2, ok2 := r.(*D.AAAA); ok2 && a2.AAAA != nil {
						ans = append(ans, a2.AAAA.String())
					}
				}
			}
			if dc.proxyAdapter != "" {
				pr = "(" + dc.proxyAdapter + ")"
			}
			log.Debug().
				Str("source", "quic://"+net.JoinHostPort(dc.host, dc.port)+pr).
				Str("qType", D.Type(q.Qtype).String()).
				Str("name", q.Name).
				Strs("answer", ans).
				Msg("[DNS] dns response")
		}
	}

	return msg, err
}

func isActive(s quic.Connection) bool {
	select {
	case <-s.Context().Done():
		return false
	default:
		return true
	}
}

func (dc *doqClient) getConnection(ctx context.Context) (quic.Connection, error) {
	var conn quic.Connection
	dc.RLock()
	conn = dc.connection
	if conn != nil && isActive(conn) {
		dc.RUnlock()
		return conn, nil
	}
	if conn != nil {
		// we're recreating the connection, let's create a new one
		_ = conn.CloseWithError(0, "")
	}
	dc.RUnlock()

	dc.Lock()
	defer dc.Unlock()

	var err error
	conn, err = dc.openConnection(ctx)
	if err != nil {
		// This does not look too nice, but QUIC (or maybe quic-go)
		// doesn't seem stable enough.
		// Maybe retransmissions aren't fully implemented in quic-go?
		// Anyway, the simple solution is to make a second try when
		// it fails to open the QUIC connection.
		conn, err = dc.openConnection(ctx)
		if err != nil {
			return nil, err
		}
	}
	dc.connection = conn
	return conn, nil
}

func (dc *doqClient) openConnection(ctx context.Context) (quic.Connection, error) {
	var err error
	if !dc.ip.IsValid() {
		if dc.r == nil {
			return nil, fmt.Errorf("dns %s not a valid ip", dc.host)
		} else {
			var ip netip.Addr
			if ip, err = resolver.ResolveIPWithResolver(ctx, dc.host, dc.r); err != nil {
				return nil, fmt.Errorf("use default dns resolve failed: %w", err)
			}
			dc.ip = ip
		}
	}

	tlsConfig := &tls.Config{
		NextProtos: []string{"doq"},
	}
	quicConfig := &quic.Config{
		HandshakeIdleTimeout: time.Second * 8,
	}

	var (
		port, _ = strconv.Atoi(dc.port)
		udpAddr = &net.UDPAddr{IP: dc.ip.AsSlice(), Port: port}
		pConn   net.PacketConn
	)
	if dc.proxyAdapter != "" {
		var conn net.Conn
		conn, err = dialContextWithProxyAdapter(ctx, dc.proxyAdapter, "udp", dc.ip, dc.port)
		if err == nil {
			pConn = conn.(net.PacketConn)
		} else if err == errProxyNotFound {
			options := []dialer.Option{dialer.WithInterface(dc.proxyAdapter), dialer.WithRoutingMark(0)}
			pConn, err = dialer.ListenPacket(ctx, "udp", "", options...)
		}
	} else {
		pConn, err = dialer.ListenPacket(ctx, "udp", "")
	}

	if err != nil {
		return nil, err
	}

	conn, err := quic.DialContext(ctx, pConn, udpAddr, dc.host, tlsConfig, quicConfig)
	if err != nil {
		return nil, err
	}

	return conn, nil
}

func (dc *doqClient) openStream(ctx context.Context) (quic.Stream, error) {
	conn, err := dc.getConnection(ctx)
	if err != nil {
		return nil, err
	}

	return conn.OpenStreamSync(ctx)
}

func newDoqClient(addr string, r *Resolver, proxyAdapter string) *doqClient {
	host, port, _ := net.SplitHostPort(addr)
	return &doqClient{
		host:         host,
		port:         port,
		r:            r,
		proxyAdapter: proxyAdapter,
	}
}
