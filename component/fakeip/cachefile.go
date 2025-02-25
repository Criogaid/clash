package fakeip

import (
	"net/netip"

	"github.com/Dreamacro/clash/component/profile/cachefile"
)

type cachefileStore struct {
	cache *cachefile.CacheFile
}

// GetByHost implements store.GetByHost
func (c *cachefileStore) GetByHost(host string) (netip.Addr, bool) {
	return netip.AddrFromSlice(c.cache.GetFakeip([]byte(host)))
}

// PutByHost implements store.PutByHost
func (c *cachefileStore) PutByHost(host string, ip netip.Addr) {
	_ = c.cache.PutFakeip([]byte(host), ip.AsSlice())
}

// GetByIP implements store.GetByIP
func (c *cachefileStore) GetByIP(ip netip.Addr) (string, bool) {
	elm := c.cache.GetFakeip(ip.AsSlice())
	if elm == nil {
		return "", false
	}
	return string(elm), true
}

// PutByIP implements store.PutByIP
func (c *cachefileStore) PutByIP(ip netip.Addr, host string) {
	_ = c.cache.PutFakeip(ip.AsSlice(), []byte(host))
}

// DelByIP implements store.DelByIP
func (c *cachefileStore) DelByIP(ip netip.Addr) {
	addr := ip.AsSlice()
	_ = c.cache.DelFakeipPair(addr, c.cache.GetFakeip(addr))
}

// Exist implements store.Exist
func (c *cachefileStore) Exist(ip netip.Addr) bool {
	_, exist := c.GetByIP(ip)
	return exist
}

// CloneTo implements store.CloneTo
// already persistence
func (c *cachefileStore) CloneTo(_ store) {}

// FlushFakeIP implements store.FlushFakeIP
func (c *cachefileStore) FlushFakeIP() error {
	return c.cache.FlushFakeIP()
}
