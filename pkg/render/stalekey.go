//go:build !ebitenginedebug

package render

// failOnStaleKey: a release takes the record made afresh and logs the
// kind once (item 212).
const failOnStaleKey = false
