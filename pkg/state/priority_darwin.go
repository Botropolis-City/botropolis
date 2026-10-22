package state

import (
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/ebitengine/purego"
	"golang.org/x/sys/unix"
)

// BackgroundPolicy is one way a background pass's thread can be put
// below the rest of the process on macOS. Item 200 measures each
// against item 138's inversion before one ships.
type BackgroundPolicy string

const (
	PolicyNone               BackgroundPolicy = "none"
	PolicyDarwinBG           BackgroundPolicy = "darwin-bg"
	PolicyQoSUtility         BackgroundPolicy = "qos-utility"
	PolicyQoSBackground      BackgroundPolicy = "qos-background"
	PolicyIOThrottle         BackgroundPolicy = "io-throttle"
	PolicyDarwinBGIOThrottle BackgroundPolicy = "darwin-bg+io-throttle"
)

// BackgroundPolicies is every policy item 200 measures.
var BackgroundPolicies = []BackgroundPolicy{
	PolicyNone, PolicyDarwinBG, PolicyQoSUtility, PolicyQoSBackground, PolicyIOThrottle, PolicyDarwinBGIOThrottle,
}

// shippedPolicy is what a pass gets unless a test chooses otherwise:
// its disk reads throttled, and its CPU priority left alone. On CI's
// Apple-silicon runner every policy in the background band froze the
// whole daemon, since the collector waits for a thread that never
// runs, and QoS utility measured the same as none there, because the
// runners already hold every thread at utility's priority (item 200).
const shippedPolicy = PolicyIOThrottle

var chosenPolicy atomic.Value

// SetBackgroundPolicy chooses the policy every later pass applies. It is
// a test seam for item 200's measurements, not a setting.
func SetBackgroundPolicy(p BackgroundPolicy) { chosenPolicy.Store(p) }

func currentPolicy() BackgroundPolicy {
	if p, ok := chosenPolicy.Load().(BackgroundPolicy); ok {
		return p
	}
	return shippedPolicy
}

// ThreadPriority is the calling thread's priority as the kernel and
// libpthread report it.
type ThreadPriority struct {
	DarwinBG     bool
	QoS          uint32
	DiskIO       int
	BasePriority int32
}

const (
	prioDarwinThread   = 3
	prioDarwinBG       = 0x1000
	qosClassUtility    = 0x11
	qosClassBackground = 0x09
	iopolTypeDisk      = 0
	iopolScopeThread   = 1
	iopolThrottle      = 3
	threadExtendedInfo = 5
)

type threadExtendedInfoData struct {
	UserTime, SystemTime                uint64
	CPUUsage, Policy, RunState, Flags   int32
	SleepTime, CurPri, Priority, MaxPri int32
	Name                                [64]byte
}

var libSystem struct {
	once            sync.Once
	ok              bool
	setQoSClassSelf func(qos uint32, relpri int32) int32
	qosClassSelf    func() uint32
	setIOPolicy     func(iotype, scope, policy int32) int32
	getIOPolicy     func(iotype, scope int32) int32
	pthreadSelf     func() uintptr
	machThread      func(pthread uintptr) uint32
	threadInfo      func(thread uint32, flavor int32, info *threadExtendedInfoData, count *uint32) int32
}

func loadLibSystem() bool {
	l := &libSystem
	l.once.Do(func() {
		h, err := purego.Dlopen("/usr/lib/libSystem.B.dylib", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			return
		}
		purego.RegisterLibFunc(&l.setQoSClassSelf, h, "pthread_set_qos_class_self_np")
		purego.RegisterLibFunc(&l.qosClassSelf, h, "qos_class_self")
		purego.RegisterLibFunc(&l.setIOPolicy, h, "setiopolicy_np")
		purego.RegisterLibFunc(&l.getIOPolicy, h, "getiopolicy_np")
		purego.RegisterLibFunc(&l.pthreadSelf, h, "pthread_self")
		purego.RegisterLibFunc(&l.machThread, h, "pthread_mach_thread_np")
		purego.RegisterLibFunc(&l.threadInfo, h, "thread_info")
		l.ok = true
	})
	return l.ok
}

func setDarwinBG() {
	_ = unix.Setpriority(prioDarwinThread, 0, prioDarwinBG)
}

// backgroundPriority puts the calling thread below the rest of the
// process under the chosen policy.
func backgroundPriority() {
	p := currentPolicy()
	if p != PolicyNone && p != PolicyDarwinBG && !loadLibSystem() {
		return
	}
	switch p {
	case PolicyDarwinBG:
		setDarwinBG()
	case PolicyQoSUtility:
		libSystem.setQoSClassSelf(qosClassUtility, 0)
	case PolicyQoSBackground:
		libSystem.setQoSClassSelf(qosClassBackground, 0)
	case PolicyIOThrottle:
		libSystem.setIOPolicy(iopolTypeDisk, iopolScopeThread, iopolThrottle)
	case PolicyDarwinBGIOThrottle:
		setDarwinBG()
		libSystem.setIOPolicy(iopolTypeDisk, iopolScopeThread, iopolThrottle)
	}
}

// ThreadPolicy reads the calling thread's priority. It is a test seam
// for item 200, so a measurement can show its policy took effect.
func ThreadPolicy() ThreadPriority {
	var p ThreadPriority
	if bg, err := unix.Getpriority(prioDarwinThread, 0); err == nil {
		p.DarwinBG = bg != 0
	}
	if !loadLibSystem() {
		return p
	}
	p.QoS = libSystem.qosClassSelf()
	p.DiskIO = int(libSystem.getIOPolicy(iopolTypeDisk, iopolScopeThread))
	thread := libSystem.machThread(libSystem.pthreadSelf())
	var info threadExtendedInfoData
	count := uint32(unsafe.Sizeof(info) / 4)
	if libSystem.threadInfo(thread, threadExtendedInfo, &info, &count) == 0 {
		p.BasePriority = info.Priority
	}
	return p
}
