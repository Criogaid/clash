package adapter

import (
	"fmt"
	"strings"

	"github.com/Dreamacro/clash/adapter/outbound"
	"github.com/Dreamacro/clash/common/structure"
	"github.com/Dreamacro/clash/common/util"
	C "github.com/Dreamacro/clash/constant"
)

func ParseProxy(mapping map[string]any, forceCertVerify, udp, autoCipher, randomHost, disableDNS bool) (C.Proxy, error) {
	decoder := structure.NewDecoder(structure.Option{TagName: "proxy", WeaklyTypedInput: true})
	proxyType, existType := mapping["type"].(string)
	if !existType {
		return nil, fmt.Errorf("missing type")
	}

	var (
		proxy C.ProxyAdapter
		err   error
	)
	switch proxyType {
	case "ss":
		ssOption := &outbound.ShadowSocksOption{RemoteDnsResolve: true}
		err = decoder.Decode(mapping, ssOption)
		if err != nil {
			break
		}
		if udp {
			ssOption.UDP = true
		}
		if randomHost {
			ssOption.RandomHost = true
		}
		if disableDNS {
			ssOption.RemoteDnsResolve = false
		}
		proxy, err = outbound.NewShadowSocks(*ssOption)
	case "ssr":
		ssrOption := &outbound.ShadowSocksROption{RemoteDnsResolve: true}
		err = decoder.Decode(mapping, ssrOption)
		if err != nil {
			break
		}
		if udp {
			ssrOption.UDP = true
		}
		if randomHost {
			ssrOption.RandomHost = true
		}
		if disableDNS {
			ssrOption.RemoteDnsResolve = false
		}
		proxy, err = outbound.NewShadowSocksR(*ssrOption)
	case "socks5":
		socksOption := &outbound.Socks5Option{RemoteDnsResolve: true}
		err = decoder.Decode(mapping, socksOption)
		if err != nil {
			break
		}
		if forceCertVerify {
			socksOption.SkipCertVerify = false
		}
		if udp {
			socksOption.UDP = true
		}
		if disableDNS {
			socksOption.RemoteDnsResolve = false
		}
		proxy = outbound.NewSocks5(*socksOption)
	case "http":
		httpOption := &outbound.HttpOption{RemoteDnsResolve: true}
		err = decoder.Decode(mapping, httpOption)
		if err != nil {
			break
		}
		if forceCertVerify {
			httpOption.SkipCertVerify = false
		}
		if disableDNS {
			httpOption.RemoteDnsResolve = false
		}
		proxy = outbound.NewHttp(*httpOption)
	case "vmess":
		vmessOption := &outbound.VmessOption{
			HTTPOpts: outbound.HTTPOptions{
				Method:  "GET",
				Path:    []string{"/"},
				Headers: make(map[string][]string),
			},
			RemoteDnsResolve: true,
		}
		err = decoder.Decode(mapping, vmessOption)
		if err != nil {
			break
		}
		vmessOption.HTTPOpts.Method = util.EmptyOr(strings.ToUpper(vmessOption.HTTPOpts.Method), "GET")
		if forceCertVerify {
			vmessOption.SkipCertVerify = false
		}
		if udp {
			vmessOption.UDP = true
		}
		if autoCipher {
			vmessOption.Cipher = "auto"
		}
		if randomHost {
			vmessOption.RandomHost = true
		}
		if disableDNS {
			vmessOption.RemoteDnsResolve = false
		}
		proxy, err = outbound.NewVmess(*vmessOption)
	case "vless":
		vlessOption := &outbound.VlessOption{
			HTTPOpts: outbound.HTTPOptions{
				Method:  "GET",
				Path:    []string{"/"},
				Headers: make(map[string][]string),
			},
			RemoteDnsResolve: true,
		}
		err = decoder.Decode(mapping, vlessOption)
		if err != nil {
			break
		}
		vlessOption.HTTPOpts.Method = util.EmptyOr(strings.ToUpper(vlessOption.HTTPOpts.Method), "GET")
		if forceCertVerify {
			vlessOption.SkipCertVerify = false
		}
		if udp {
			vlessOption.UDP = true
		}
		if disableDNS {
			vlessOption.RemoteDnsResolve = false
		}
		proxy, err = outbound.NewVless(*vlessOption)
	case "snell":
		snellOption := &outbound.SnellOption{RemoteDnsResolve: true}
		err = decoder.Decode(mapping, snellOption)
		if err != nil {
			break
		}
		if udp {
			snellOption.UDP = true
		}
		if randomHost {
			snellOption.RandomHost = true
		}
		if disableDNS {
			snellOption.RemoteDnsResolve = false
		}
		proxy, err = outbound.NewSnell(*snellOption)
	case "trojan":
		trojanOption := &outbound.TrojanOption{RemoteDnsResolve: true}
		err = decoder.Decode(mapping, trojanOption)
		if err != nil {
			break
		}
		if forceCertVerify {
			trojanOption.SkipCertVerify = false
		}
		if udp {
			trojanOption.UDP = true
		}
		if disableDNS {
			trojanOption.RemoteDnsResolve = false
		}
		proxy, err = outbound.NewTrojan(*trojanOption)
	case "wireguard":
		wireguardOption := &outbound.WireGuardOption{
			RemoteDnsResolve: true,
		}
		err = decoder.Decode(mapping, wireguardOption)
		if err != nil {
			break
		}
		if udp {
			wireguardOption.UDP = true
		}
		proxy, err = outbound.NewWireGuard(*wireguardOption)
	default:
		return nil, fmt.Errorf("unsupport proxy type: %s", proxyType)
	}

	if err != nil {
		return nil, err
	}

	return NewProxy(proxy), nil
}
