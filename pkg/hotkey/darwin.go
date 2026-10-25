//go:build darwin

package hotkey

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"

	"botropolis.city/botropolis/pkg/config"
)

// Activation is the call that brings the window forward. Since macOS 14
// activating is a request the system may refuse, and which of these it
// honours for a hot key the application itself received is what the
// macOS job finds out. Item 95.
type Activation int

const (
	// Activate is NSApplication's activate, macOS 14's cooperative call.
	Activate Activation = iota
	// ActivateIgnoringOtherApps is the call activate replaced, deprecated in 14.
	ActivateIgnoringOtherApps
	// SetFrontProcess is Carbon's deprecated SetFrontProcessWithOptions.
	SetFrontProcess
)

func (a Activation) String() string {
	switch a {
	case Activate:
		return "activate"
	case ActivateIgnoringOtherApps:
		return "activateIgnoringOtherApps"
	case SetFrontProcess:
		return "SetFrontProcessWithOptions"
	}
	return fmt.Sprintf("Activation(%d)", int(a))
}

const (
	hotKeySignature           = 0x62747270 // 'btrp'
	eventClassKeyboard        = 0x6b657962 // 'keyb'
	eventHotKeyPressed        = 5
	setFrontProcessWindowOnly = 1
	eventHotKeyInvalidErr     = -9868
	eventHotKeyExistsErr      = -9878
	carbonPath                = "/System/Library/Frameworks/Carbon.framework/Carbon"
	applicationServicesPath   = "/System/Library/Frameworks/ApplicationServices.framework/ApplicationServices"
	appKitPath                = "/System/Library/Frameworks/AppKit.framework/AppKit"
	libSystemPath             = "/usr/lib/libSystem.B.dylib"
)

type processSerialNumber struct{ high, low uint32 }

var mac struct {
	once sync.Once
	err  error

	appTarget  func() uintptr
	install    func(target, handler, count uintptr, specs *[2]uint32, user uintptr, out *uintptr) int32
	register   func(code, mods uint32, id uint64, target uintptr, options uint32, out *uintptr) int32
	unregister func(ref uintptr) int32
	setFront   func(psn *processSerialNumber, options uint32) int32
	process    func(psn *processSerialNumber) int32

	isMain     func() int32
	asyncF     func(queue, context, work uintptr)
	syncF      func(queue, context, work uintptr)
	mainQueue  uintptr
	trampoline uintptr

	handler uintptr

	mu      sync.Mutex
	next    uintptr
	pending map[uintptr]func()
}

// current is the desktop the one hot key handler delivers to.
var current atomic.Pointer[macOS]

func bind(handle uintptr, fn any, name string) error {
	sym, err := purego.Dlsym(handle, name)
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	purego.RegisterFunc(fn, sym)
	return nil
}

func loadMac() error {
	mac.once.Do(func() {
		handles := map[string]uintptr{}
		for _, path := range []string{carbonPath, applicationServicesPath, appKitPath, libSystemPath} {
			h, err := purego.Dlopen(path, purego.RTLD_NOW|purego.RTLD_GLOBAL)
			if err != nil {
				mac.err = fmt.Errorf("%w (%v); %s", ErrUnsupported, err, Fallback)
				return
			}
			handles[path] = h
		}
		for _, b := range []struct {
			lib  string
			fn   any
			name string
		}{
			{carbonPath, &mac.appTarget, "GetApplicationEventTarget"},
			{carbonPath, &mac.install, "InstallEventHandler"},
			{carbonPath, &mac.register, "RegisterEventHotKey"},
			{carbonPath, &mac.unregister, "UnregisterEventHotKey"},
			{applicationServicesPath, &mac.setFront, "SetFrontProcessWithOptions"},
			{applicationServicesPath, &mac.process, "GetCurrentProcess"},
			{libSystemPath, &mac.isMain, "pthread_main_np"},
			{libSystemPath, &mac.asyncF, "dispatch_async_f"},
			{libSystemPath, &mac.syncF, "dispatch_sync_f"},
		} {
			if err := bind(handles[b.lib], b.fn, b.name); err != nil {
				mac.err = fmt.Errorf("%w (%v); %s", ErrUnsupported, err, Fallback)
				return
			}
		}
		q, err := purego.Dlsym(handles[libSystemPath], "_dispatch_main_q")
		if err != nil {
			mac.err = fmt.Errorf("%w (%v); %s", ErrUnsupported, err, Fallback)
			return
		}
		mac.mainQueue = q
		mac.pending = map[uintptr]func(){}
		mac.trampoline = purego.NewCallback(func(context uintptr) uintptr {
			mac.mu.Lock()
			f := mac.pending[context]
			delete(mac.pending, context)
			mac.mu.Unlock()
			if f != nil {
				f()
			}
			return 0
		})
		mac.handler = purego.NewCallback(func(_, _, _ uintptr) uintptr {
			if d := current.Load(); d != nil {
				select {
				case d.fired <- uint32(time.Now().UnixMilli()):
				default:
				}
			}
			return 0
		})
		var installed uintptr
		mac.err = onMainWait(func() error {
			specs := [2]uint32{eventClassKeyboard, eventHotKeyPressed}
			if status := mac.install(mac.appTarget(), mac.handler, 1, &specs, 0, &installed); status != 0 {
				return fmt.Errorf("%w (InstallEventHandler returned %d); %s", ErrUnsupported, status, Fallback)
			}
			return nil
		})
	})
	return mac.err
}

func queued(f func()) uintptr {
	mac.mu.Lock()
	defer mac.mu.Unlock()
	mac.next++
	mac.pending[mac.next] = f
	return mac.next
}

// onMainWait runs f on the main thread and waits for it. Ebitengine's
// main thread drains the main queue only while it polls events, between
// frames, so this is for before the city runs, never from Update.
func onMainWait(f func() error) error {
	if mac.isMain() != 0 {
		return f()
	}
	var err error
	mac.syncF(mac.mainQueue, queued(func() { err = f() }), mac.trampoline)
	return err
}

// onMain runs f on the main thread when it next drains the main queue,
// without waiting.
func onMain(f func()) {
	if mac.isMain() != 0 {
		f()
		return
	}
	mac.asyncF(mac.mainQueue, queued(f), mac.trampoline)
}

// macOS registers one combination through Carbon's RegisterEventHotKey,
// which needs no Accessibility permission, and comes forward with how.
type macOS struct {
	how   Activation
	fired chan uint32

	mu  sync.Mutex
	ref uintptr
}

// OpenMacOS is the macOS desktop, coming forward with how. It installs
// the hot key handler on the main thread, so it is called before the
// city runs, or while the main thread runs its loop.
func OpenMacOS(how Activation) (Desktop, error) {
	if err := loadMac(); err != nil {
		return nil, err
	}
	d := &macOS{how: how, fired: make(chan uint32, 1)}
	current.Store(d)
	return d, nil
}

func (d *macOS) Fired() <-chan uint32 { return d.fired }

func (d *macOS) Grab(c config.Chord) error {
	if err := RefusedOn("darwin", c); err != nil {
		return err
	}
	k, err := carbonChord(c)
	if err != nil {
		return err
	}
	if err := d.Release(); err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	var ref uintptr
	status := mac.register(k.code, k.mods, uint64(hotKeySignature)|1<<32, mac.appTarget(), 0, &ref)
	switch status {
	case 0:
		d.ref = ref
		return nil
	case eventHotKeyInvalidErr:
		return fmt.Errorf("%w: macOS will not register %s", ErrRefused, c)
	case eventHotKeyExistsErr:
		return fmt.Errorf("%w %s; choose another, or %s", ErrHeld, c, Fallback)
	}
	return fmt.Errorf("%s: RegisterEventHotKey returned %d", c, status)
}

func (d *macOS) Release() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.ref == 0 {
		return nil
	}
	status := mac.unregister(d.ref)
	d.ref = 0
	if status != 0 {
		return fmt.Errorf("UnregisterEventHotKey returned %d", status)
	}
	return nil
}

// Raise asks for the window to come forward, on the main thread at its
// next drain; whether the system honours it is the system's call.
func (d *macOS) Raise(uint32) bool {
	how := d.how
	onMain(func() {
		switch how {
		case SetFrontProcess:
			var psn processSerialNumber
			if mac.process(&psn) == 0 {
				mac.setFront(&psn, setFrontProcessWindowOnly)
			}
		default:
			app := objc.ID(objc.GetClass("NSApplication")).Send(objc.RegisterName("sharedApplication"))
			if how == Activate && objc.Send[bool](app, objc.RegisterName("respondsToSelector:"), objc.RegisterName("activate")) {
				app.Send(objc.RegisterName("activate"))
				return
			}
			app.Send(objc.RegisterName("activateIgnoringOtherApps:"), true)
		}
	})
	return true
}

func (d *macOS) Close() {
	_ = d.Release()
	current.CompareAndSwap(d, nil)
}
