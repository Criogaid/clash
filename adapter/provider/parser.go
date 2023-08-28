package provider

import (
	"errors"
	"fmt"
	"time"

	"github.com/Dreamacro/clash/common/structure"
	C "github.com/Dreamacro/clash/constant"
	types "github.com/Dreamacro/clash/constant/provider"
)

var errVehicleType = errors.New("unsupport vehicle type")

type healthCheckSchema struct {
	Enable   bool          `provider:"enable"`
	URL      string        `provider:"url"`
	Interval time.Duration `provider:"interval"`
	Lazy     bool          `provider:"lazy,omitempty"`
}

type proxyProviderSchema struct {
	Type            string              `provider:"type"`
	Path            string              `provider:"path"`
	URL             string              `provider:"url,omitempty"`
	URLProxy        bool                `provider:"url-proxy,omitempty"`
	Interval        time.Duration       `provider:"interval,omitempty"`
	Filter          string              `provider:"filter,omitempty"`
	HealthCheck     healthCheckSchema   `provider:"health-check,omitempty"`
	ForceCertVerify bool                `provider:"force-cert-verify,omitempty"`
	UDP             bool                `provider:"udp,omitempty"`
	RandomHost      bool                `provider:"rand-host,omitempty"`
	DisableDNS      bool                `provider:"disable-dns,omitempty"`
	PrefixName      string              `provider:"prefix-name,omitempty"`
	Header          map[string][]string `provider:"header,omitempty"`
}

func ParseProxyProvider(name string, mapping map[string]any, forceCertVerify bool) (types.ProxyProvider, error) {
	decoder := structure.NewDecoder(structure.Option{TagName: "provider", WeaklyTypedInput: true})

	schema := &proxyProviderSchema{
		HealthCheck: healthCheckSchema{
			Lazy: true,
		},
	}

	if forceCertVerify {
		schema.ForceCertVerify = true
	}

	if schema.Interval < 0 {
		schema.Interval = 0
	}

	if schema.HealthCheck.Interval < 0 {
		schema.HealthCheck.Interval = 0
	}

	if err := decoder.Decode(mapping, schema); err != nil {
		return nil, err
	}

	var hcInterval time.Duration
	if schema.HealthCheck.Enable {
		hcInterval = schema.HealthCheck.Interval
	}
	hc := NewHealthCheck([]C.Proxy{}, schema.HealthCheck.URL, hcInterval, schema.HealthCheck.Lazy)

	vehicle, err := newVehicle(schema)
	if err != nil {
		return nil, err
	}

	interval := schema.Interval
	filter := schema.Filter
	return NewProxySetProvider(name, interval, filter, vehicle, hc, schema.ForceCertVerify,
		schema.UDP, schema.RandomHost, schema.DisableDNS, schema.PrefixName)
}

func newVehicle(schema *proxyProviderSchema) (types.Vehicle, error) {
	path := C.Path.Resolve(schema.Path)

	switch schema.Type {
	case "file":
		return NewFileVehicle(path), nil
	case "http":
		if !C.Path.IsSubHomeDir(path) {
			return nil, errors.New("the path is not a sub path of home directory")
		}

		if schema.Header == nil {
			schema.Header = map[string][]string{
				"User-Agent": {"ClashPlusPro/" + C.Version},
			}
		} else if _, ok := schema.Header["User-Agent"]; !ok {
			schema.Header["User-Agent"] = []string{"ClashPlusPro/" + C.Version}
		}

		return NewHTTPVehicle(path, schema.URL, schema.URLProxy, schema.Header), nil
	default:
		return nil, fmt.Errorf("%w: %s", errVehicleType, schema.Type)
	}
}
