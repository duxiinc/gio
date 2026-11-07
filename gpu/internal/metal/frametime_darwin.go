// SPDX-License-Identifier: Unlicense OR MIT

package metal

/*
#include <stdint.h>
*/
import "C"

import (
	"time"

	"gioui.org/gpu/frametime"
)

// gioMetalFrameTimed runs on a Metal completion thread. Times are host
// seconds; now is the host time of the call, to map them onto time.Time.
// start and end are 0 where the GPU timestamps are unavailable.
//
//export gioMetalFrameTimed
func gioMetalFrameTimed(target C.uintptr_t, commit, start, end, now C.double) {
	n := time.Now()
	at := func(t C.double) time.Time {
		if t <= 0 {
			return time.Time{}
		}
		return n.Add(-time.Duration(float64(now-t) * float64(time.Second)))
	}
	frametime.Report(frametime.Frame{
		Target:   uintptr(target),
		Commit:   at(commit),
		GPUStart: at(start),
		GPUEnd:   at(end),
	})
}
