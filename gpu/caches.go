// SPDX-License-Identifier: Unlicense OR MIT

package gpu

import (
	"fmt"
	"image"

	"gioui.org/internal/f32"
)

type textureCacheKey struct {
	filter byte
	handle any
}

type textureCache struct {
	res map[textureCacheKey]textureCacheValue
	// spare holds image textures evicted by the last frame. A streamed image
	// gets a new ImageOp handle every frame, so without this every frame
	// would allocate a texture of the same size.
	spare []*texture
}

type textureCacheValue struct {
	used bool
	idle int
	tex  *texture
}

// sharedIdleFrames is how many frames a lockable shared texture stays open
// unpainted. Producers rotate a few slots, so each comes back within a few
// frames and reopening it would cost a resource open and a new view.
const sharedIdleFrames = 8

const maxSpareTextures = 2

// opCache is like a resourceCache but using concrete types and a
// freelist instead of two maps to avoid runtime.mapaccess2 calls
// since benchmarking showed them as a bottleneck.
type opCache struct {
	// store the index + 1 in cache this key is stored in
	index map[opKey]int
	// list of indexes in cache that are free and can be used
	freelist []int
	cache    []opCacheValue
}

type opCacheValue struct {
	data pathData

	bounds f32.Rectangle
	// the fields below are handled by opCache
	key  opKey
	keep bool
}

func newTextureCache() *textureCache {
	return &textureCache{
		res: make(map[textureCacheKey]textureCacheValue),
	}
}

func (r *textureCache) get(key textureCacheKey) (*texture, bool) {
	v, exists := r.res[key]
	if !exists {
		return nil, false
	}
	if !v.used {
		v.used = true
		v.idle = 0
		r.res[key] = v
	}
	return v.tex, true
}

func (r *textureCache) put(key textureCacheKey, tex *texture) {
	v, exists := r.res[key]
	if exists && v.used {
		panic(fmt.Errorf("key exists, %v", key))
	}
	r.res[key] = textureCacheValue{used: true, tex: tex}
}

func (r *textureCache) drop(key textureCacheKey) {
	if v, ok := r.res[key]; ok {
		delete(r.res, key)
		v.tex.release()
	}
}

// takeSpare returns an evicted image texture of the given size and filter.
func (r *textureCache) takeSpare(size image.Point, filter byte) *texture {
	for i, t := range r.spare {
		if t.size == size && t.filter == filter {
			last := len(r.spare) - 1
			r.spare[i] = r.spare[last]
			r.spare[last] = nil
			r.spare = r.spare[:last]
			return t
		}
	}
	return nil
}

func (r *textureCache) frame() {
	for _, t := range r.spare {
		t.release()
	}
	clear(r.spare)
	r.spare = r.spare[:0]
	for k, v := range r.res {
		switch {
		case v.used:
			if v.tex.lock != nil {
				v.tex.lock.ReleaseShared()
			}
			v.used = false
			r.res[k] = v
		case v.tex.lock != nil && v.idle < sharedIdleFrames:
			v.idle++
			r.res[k] = v
		default:
			delete(r.res, k)
			if v.tex.src != nil && len(r.spare) < maxSpareTextures {
				v.tex.src = nil
				r.spare = append(r.spare, v.tex)
			} else {
				v.tex.release()
			}
		}
	}
}

func (r *textureCache) release() {
	for _, v := range r.res {
		v.tex.release()
	}
	for _, t := range r.spare {
		t.release()
	}
	r.res = nil
	r.spare = nil
}

func newOpCache() *opCache {
	return &opCache{
		index:    make(map[opKey]int),
		freelist: make([]int, 0),
		cache:    make([]opCacheValue, 0),
	}
}

func (r *opCache) get(key opKey) (o opCacheValue, exist bool) {
	v := r.index[key]
	if v == 0 {
		return
	}
	r.cache[v-1].keep = true
	return r.cache[v-1], true
}

func (r *opCache) put(key opKey, val opCacheValue) {
	v := r.index[key]
	val.keep = true
	val.key = key
	if v == 0 {
		// not in cache
		i := len(r.cache)
		if len(r.freelist) > 0 {
			i = r.freelist[len(r.freelist)-1]
			r.freelist = r.freelist[:len(r.freelist)-1]
			r.cache[i] = val
		} else {
			r.cache = append(r.cache, val)
		}
		r.index[key] = i + 1
	} else {
		r.cache[v-1] = val
	}
}

func (r *opCache) frame() {
	r.freelist = r.freelist[:0]
	for i, v := range r.cache {
		r.cache[i].keep = false
		if v.keep {
			continue
		}
		if v.data.data != nil {
			v.data.release()
			r.cache[i].data.data = nil
		}
		delete(r.index, v.key)
		r.freelist = append(r.freelist, i)
	}
}

func (r *opCache) release() {
	for i := range r.cache {
		r.cache[i].keep = false
	}
	r.frame()
	r.index = nil
	r.freelist = nil
	r.cache = nil
}
