// SPDX-License-Identifier: Unlicense OR MIT

// Package frametime reports when the GPU ran each window frame. Only the
// Metal backend reports; elsewhere the hook is never called.
package frametime

import (
	"sync/atomic"
	"time"
)

// Frame is the GPU timing of one frame's final command buffer.
type Frame struct {
	// Target is the frame's render target. On Metal it is the MTLTexture of
	// the window drawable, the same object as CAMetalDrawable.texture.
	Target uintptr
	// Commit is when the command buffer was committed, GPUStart and GPUEnd
	// when the GPU started and finished executing it.
	Commit, GPUStart, GPUEnd time.Time
}

var hook atomic.Pointer[func(Frame)]

// SetHook installs f to receive every timed frame, on a GPU completion
// thread. f must not block. A nil f stops reporting.
func SetHook(f func(Frame)) {
	if f == nil {
		hook.Store(nil)
		return
	}
	hook.Store(&f)
}

// Enabled reports whether a hook is installed, so backends can skip timing.
func Enabled() bool { return hook.Load() != nil }

// Report delivers f to the hook. It is called by GPU backends.
func Report(f Frame) {
	if h := hook.Load(); h != nil {
		(*h)(f)
	}
}
