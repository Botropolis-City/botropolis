package fonts_test

import (
	"bytes"
	"testing"

	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/assets/fonts"
)

func TestInter(t *testing.T) {
	t.Run("when the embedded face is read", func(t *testing.T) {
		data, err := fonts.Inter()
		require.NoError(t, err)

		t.Run("it should parse as a font", func(t *testing.T) {
			_, err := text.NewGoTextFaceSource(bytes.NewReader(data))
			assert.NoError(t, err)
		})
	})

	t.Run("when the licence is read", func(t *testing.T) {
		licence, err := fonts.License()
		require.NoError(t, err)

		t.Run("it should be the SIL Open Font License", func(t *testing.T) {
			assert.Contains(t, licence, "SIL Open Font License")
		})
	})
}

func TestGoMono(t *testing.T) {
	t.Run("when the embedded monospace face is read", func(t *testing.T) {
		source, err := text.NewGoTextFaceSource(bytes.NewReader(fonts.GoMono()))
		require.NoError(t, err)
		face := &text.GoTextFace{Source: source, Size: 14}

		t.Run("it should give a narrow and a wide letter the same advance", func(t *testing.T) {
			narrow, _ := text.Measure("iiii", face, 0)
			wide, _ := text.Measure("mmmm", face, 0)
			assert.Equal(t, narrow, wide)
		})
	})

	t.Run("when the embedded bold monospace face is read", func(t *testing.T) {
		source, err := text.NewGoTextFaceSource(bytes.NewReader(fonts.GoMonoBold()))
		require.NoError(t, err)
		face := &text.GoTextFace{Source: source, Size: 14}

		t.Run("it should give a narrow and a wide letter the same advance", func(t *testing.T) {
			narrow, _ := text.Measure("iiii", face, 0)
			wide, _ := text.Measure("mmmm", face, 0)
			assert.Equal(t, narrow, wide)
		})
	})

	t.Run("when its licence is read", func(t *testing.T) {
		licence, err := fonts.GoMonoLicense()
		require.NoError(t, err)

		t.Run("it should be Bigelow & Holmes' BSD licence", func(t *testing.T) {
			assert.Contains(t, licence, "Bigelow & Holmes")
		})
	})
}
