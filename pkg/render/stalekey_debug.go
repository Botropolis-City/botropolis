//go:build ebitenginedebug

package render

// failOnStaleKey: a debug build fails loudly on a kept record that
// differs from the one made afresh (item 212).
const failOnStaleKey = true
