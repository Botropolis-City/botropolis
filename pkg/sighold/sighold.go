// Package sighold keeps asynchronous signals off the thread that runs the
// city's loop (item 128).
//
// Ebitengine opens every /dev/input/event* from its main thread, and evdev
// takes the device's mutex with mutex_lock_interruptible, which fails with
// EINTR, never restarted, when a signal arrives while another process holds
// it. Ebitengine returns that error from the loop, so it ended the city.
// A signal blocked on that thread cannot interrupt the open: a
// process-directed one is delivered to another thread, and Go's own
// thread-directed preemption signal waits, falling back to cooperative
// preemption there.
package sighold
