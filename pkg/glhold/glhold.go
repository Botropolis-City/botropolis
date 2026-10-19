// Package glhold keeps the GL libraries a process is already using loaded
// until it exits (item 125).
//
// Ebitengine keeps its GL context current on its render thread and
// terminates GLFW on the main thread. EGL and GLX defer destroying a
// context that is current on another thread, so llvmpipe is never torn
// down and its rasterizer threads may still be drawing the last frame
// when GLFW dlcloses the library, which unmaps their code under them.
// A library opened with RTLD_NODELETE is never unmapped, so those threads
// finish harmlessly and die with the process.
package glhold

// GL is every library GLFW's EGL and GLX paths load, through glvnd or
// not. Pinning Mesa's vendor libraries pins libgallium too, which they
// name as a dependency, so its versioned file name is never needed.
var GL = []string{"libEGL.so.1", "libEGL_mesa.so.0", "libGLESv2.so.2", "libGL.so.1", "libGLX.so.0", "libGLX_mesa.so.0"}
