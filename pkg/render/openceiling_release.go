//go:build !devtools

package render

// frameCeiling is always the whole wait outside a devtools build: a real
// capture never films a partial read, however slowly it advances.
const frameCeiling = totalCeiling
