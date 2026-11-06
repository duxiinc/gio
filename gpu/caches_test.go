// SPDX-License-Identifier: Unlicense OR MIT

package gpu

import (
	"image"
	"testing"
)

type fakeLock struct{ acquired, released int }

func (l *fakeLock) AcquireShared() error { l.acquired++; return nil }
func (l *fakeLock) ReleaseShared()       { l.released++ }

func TestTextureCacheReusesEvictedImageTexture(t *testing.T) {
	c := newTextureCache()
	size := image.Pt(4, 4)
	first := &texture{src: image.NewRGBA(image.Rect(0, 0, 4, 4)), size: size}
	c.put(textureCacheKey{handle: new(int)}, first)
	c.frame()
	if got := c.takeSpare(size, 0); got != nil {
		t.Fatal("a texture painted last frame must not be a spare yet")
	}
	c.frame()
	if got := c.takeSpare(image.Pt(8, 8), 0); got != nil {
		t.Fatal("spare reused for a different size")
	}
	if got := c.takeSpare(size, filterNearest); got != nil {
		t.Fatal("spare reused for a different filter")
	}
	if got := c.takeSpare(size, 0); got != first {
		t.Fatalf("spare = %p, want %p", got, first)
	}
}

func TestTextureCacheDropsUnusedSpares(t *testing.T) {
	c := newTextureCache()
	size := image.Pt(4, 4)
	c.put(textureCacheKey{handle: new(int)}, &texture{src: image.NewRGBA(image.Rect(0, 0, 4, 4)), size: size})
	c.frame()
	c.frame()
	c.frame()
	if got := c.takeSpare(size, 0); got != nil {
		t.Fatal("spare survived a frame without being reused")
	}
}

func TestTextureCacheKeepsLockedSharedTextureOpen(t *testing.T) {
	c := newTextureCache()
	lock := &fakeLock{}
	key := textureCacheKey{handle: uintptr(1)}
	c.put(key, &texture{lock: lock})
	c.frame()
	if lock.released != 1 {
		t.Fatalf("released = %d after a painted frame, want 1", lock.released)
	}
	for range sharedIdleFrames {
		c.frame()
	}
	if _, ok := c.get(key); !ok {
		t.Fatal("shared texture closed before the idle limit")
	}
	c.frame()
	for range sharedIdleFrames + 1 {
		c.frame()
	}
	if _, ok := c.res[key]; ok {
		t.Fatal("idle shared texture still open")
	}
	if lock.released != 2 {
		t.Fatalf("released = %d, want one per painted frame", lock.released)
	}
}
