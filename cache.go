package link_preview

import "github.com/47Billion/link-preview/oembed"

type cacheGetFunc func(string) *oembed.Info
type cacheSetFunc func(*oembed.Info, int64)

type cache struct {
	getHandler cacheGetFunc
	setHandler cacheSetFunc
	ttl        int64
}

func (c *cache) HandleCacheGet(f cacheGetFunc) {
	c.getHandler = f
}

func (c *cache) HandleCacheSet(f cacheSetFunc) {
	c.setHandler = f
}

func NewCache(ttl int64) *cache {
	c := &cache{
		ttl:        ttl,
		getHandler: func(string) *oembed.Info { return nil },
		setHandler: func(*oembed.Info, int64) {},
	}

	return c
}
