package reporting

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
	"testing"

	"github.com/go-pdf/fpdf"
)

func approxEqual(a, b float64) bool {
	const eps = 1e-6
	return math.Abs(a-b) <= eps
}

// encodeLogoTestPNG returns a deterministic black-opaque RGBA PNG of the
// requested pixel dimensions. Used to drive scaledLogoSize with a real
// fpdf-registered image so the math runs against actual Width()/Height()
// values rather than a fabricated stub.
func encodeLogoTestPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, color.RGBA{R: 0, G: 0, B: 0, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png %dx%d: %v", w, h, err)
	}
	return buf.Bytes()
}

// registerLogoTestImage encodes a PNG of the given pixel size and registers
// it with a fresh fpdf instance, returning the resulting ImageInfoType.
func registerLogoTestImage(t *testing.T, w, h int, name string) *fpdf.ImageInfoType {
	t.Helper()
	pdf := fpdf.New("P", "mm", "A4", "")
	info := pdf.RegisterImageOptionsReader(
		name,
		fpdf.ImageOptions{ImageType: "png", ReadDpi: true},
		bytes.NewReader(encodeLogoTestPNG(t, w, h)),
	)
	if info == nil {
		t.Fatalf("RegisterImageOptionsReader returned nil info for %dx%d", w, h)
	}
	if info.Width() <= 0 || info.Height() <= 0 {
		t.Fatalf("registered image %dx%d reported non-positive extent w=%v h=%v",
			w, h, info.Width(), info.Height())
	}
	return info
}

func TestReportLogoTypeFromData_BranchCov(t *testing.T) {
	pngMagic8 := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
	pngTail := []byte{0, 0, 0, 13, 'I', 'H', 'D', 'R'}
	pngFull := append(append([]byte{}, pngMagic8...), pngTail...)
	jpgMagic3 := []byte{0xff, 0xd8, 0xff}
	jpgFull := append(append([]byte{}, jpgMagic3...), 0xe0, 0, 0x10, 0, 0x4a, 0x46, 0x49, 0x46)
	gif87a := []byte("GIF87a")
	gif89a := []byte("GIF89a")
	unknown := []byte("not an image magic at all")

	tests := []struct {
		name       string
		data       []byte
		configured string
		want       string
	}{
		// --- configured short-circuit arm (normalizeLogoFormat != "") ---
		{"configured png wins over jpg data", jpgFull, "png", "png"},
		{"configured jpg wins over png data", pngFull, "jpg", "jpg"},
		{"configured jpeg normalizes to jpg", pngFull, "jpeg", "jpg"},
		{"configured gif wins over png data", pngFull, "gif", "gif"},
		{"configured uppercase is normalized", jpgFull, "PNG", "png"},
		{"configured with whitespace trimmed", jpgFull, "  jpg  ", "jpg"},
		{"configured invalid falls through to png data", pngFull, "svg", "png"},
		{"configured invalid falls through to jpg data", jpgFull, "bmp", "jpg"},
		{"configured invalid falls through to gif data", gif87a, "weird", "gif"},

		// --- data-detection arms with empty configured ---
		{"empty configured + full png magic", pngFull, "", "png"},
		{"empty configured + exactly 8 png magic bytes", pngMagic8, "", "png"},
		{"empty configured + full jpg magic", jpgFull, "", "jpg"},
		{"empty configured + exactly 3 jpg magic bytes", jpgMagic3, "", "jpg"},
		{"empty configured + gif87a", gif87a, "", "gif"},
		{"empty configured + gif89a", gif89a, "", "gif"},
		{"empty configured + exactly 6 gif bytes (87a)", []byte("GIF87a"), "", "gif"},
		{"empty configured + exactly 6 gif bytes (89a)", []byte("GIF89a"), "", "gif"},

		// --- default arm (no match) ---
		{"empty configured + unknown bytes", unknown, "", ""},
		{"empty configured + nil data", nil, "", ""},
		{"empty configured + empty data", []byte{}, "", ""},
		{"png prefix too short (7 bytes)", pngMagic8[:7], "", ""},
		{"jpg prefix too short (2 bytes)", jpgMagic3[:2], "", ""},
		{"gif prefix too short (5 bytes)", gif87a[:5], "", ""},
		{"gif89a prefix wrong last char", []byte("GIF89b"), "", ""},
		{"gif87a prefix wrong last char", []byte("GIF87b"), "", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := reportLogoTypeFromData(tc.data, tc.configured)
			if got != tc.want {
				t.Fatalf("reportLogoTypeFromData(%v, %q) = %q, want %q",
					tc.data, tc.configured, got, tc.want)
			}
		})
	}
}

func TestReportLogoTypeFromPath_BranchCov(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		configured string
		want       string
	}{
		// --- configured short-circuit arm (normalizeLogoFormat != "") ---
		{"configured png wins regardless of path", "/any/thing.xyz", "png", "png"},
		{"configured jpg wins regardless of path", "/any/thing", "jpg", "jpg"},
		{"configured jpeg normalizes to jpg", "/any/thing", "jpeg", "jpg"},
		{"configured gif wins regardless of path", "/any/thing", "gif", "gif"},
		{"configured uppercase normalized to png", "/any/thing", "PNG", "png"},
		{"configured whitespace trimmed to jpg", "/any/thing", "  jpg  ", "jpg"},
		{"configured invalid falls through to extension png", "/a.png", "svg", "png"},
		{"configured invalid falls through to default arm", "/a.svg", "svg", ""},

		// --- extension switch arms with empty configured ---
		{"png extension lowercase", "logo.png", "", "png"},
		{"png extension uppercase", "LOGO.PNG", "", "png"},
		{"png extension mixed case", "Logo.PnG", "", "png"},
		{"jpg extension", "logo.jpg", "", "jpg"},
		{"jpeg extension", "logo.jpeg", "", "jpg"},
		{"JPG extension uppercase", "LOGO.JPG", "", "jpg"},
		{"JPEG extension uppercase", "LOGO.JPEG", "", "jpg"},
		{"gif extension", "logo.gif", "", "gif"},
		{"GIF extension uppercase", "LOGO.GIF", "", "gif"},

		// --- default arm (unknown / no extension) ---
		{"unknown svg extension", "logo.svg", "", ""},
		{"unknown bmp extension", "logo.bmp", "", ""},
		{"unknown webp extension", "logo.webp", "", ""},
		{"unknown txt extension", "logo.txt", "", ""},
		{"no extension at all", "README", "", ""},
		{"trailing dot only", "logo.", "", ""},
		{"trailing dot uppercased", "LOGO.", "", ""},
		{"png with trailing wrong ext", "logo.png.bak", "", ""},

		// --- realistic full paths ---
		{"absolute path png", "/var/lib/pulse/assets/brand.png", "", "png"},
		{"url-style path jpg", "https://example.com/img/logo.jpg", "", "jpg"},
		{"windows-style path gif", `C:\assets\logo.gif`, "", "gif"},
		{"empty path with empty configured", "", "", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := reportLogoTypeFromPath(tc.path, tc.configured)
			if got != tc.want {
				t.Fatalf("reportLogoTypeFromPath(%q, %q) = %q, want %q",
					tc.path, tc.configured, got, tc.want)
			}
		})
	}
}

func TestScaledLogoSize_BranchCov(t *testing.T) {
	const eps = 1e-6

	t.Run("nil_info_returns_zero_zero", func(t *testing.T) {
		w, h := scaledLogoSize(nil, 100, 100)
		if w != 0 || h != 0 {
			t.Fatalf("scaledLogoSize(nil, 100, 100) = (%v, %v), want (0, 0)", w, h)
		}
	})

	t.Run("zero_maxW_hits_scale_zero_arm", func(t *testing.T) {
		info := registerLogoTestImage(t, 8, 4, "zero-maxw")
		w, h := scaledLogoSize(info, 0, 100)
		if w != 0 || h != 0 {
			t.Fatalf("scaledLogoSize(info, 0, 100) = (%v, %v), want (0, 0)", w, h)
		}
	})

	t.Run("zero_maxH_hits_scale_zero_arm", func(t *testing.T) {
		info := registerLogoTestImage(t, 8, 4, "zero-maxh")
		w, h := scaledLogoSize(info, 100, 0)
		if w != 0 || h != 0 {
			t.Fatalf("scaledLogoSize(info, 100, 0) = (%v, %v), want (0, 0)", w, h)
		}
	})

	t.Run("negative_maxW_hits_scale_zero_arm", func(t *testing.T) {
		info := registerLogoTestImage(t, 8, 4, "neg-maxw")
		w, h := scaledLogoSize(info, -10, 100)
		if w != 0 || h != 0 {
			t.Fatalf("scaledLogoSize(info, -10, 100) = (%v, %v), want (0, 0)", w, h)
		}
	})

	t.Run("negative_maxH_hits_scale_zero_arm", func(t *testing.T) {
		info := registerLogoTestImage(t, 8, 4, "neg-maxh")
		w, h := scaledLogoSize(info, 100, -10)
		if w != 0 || h != 0 {
			t.Fatalf("scaledLogoSize(info, 100, -10) = (%v, %v), want (0, 0)", w, h)
		}
	})

	t.Run("square_image_square_box_fills_both_axes", func(t *testing.T) {
		info := registerLogoTestImage(t, 4, 4, "square")
		const maxW, maxH = 100.0, 100.0
		w, h := scaledLogoSize(info, maxW, maxH)
		if !approxEqual(w, 100) || !approxEqual(h, 100) {
			t.Fatalf("scaledLogoSize(square 4x4, 100, 100) = (%v, %v), want (100, 100)", w, h)
		}
	})

	t.Run("wide_image_in_square_box_width_binds", func(t *testing.T) {
		// 8x4 aspect 2:1 in a 1:1 box: width binds, output is (100, 50).
		info := registerLogoTestImage(t, 8, 4, "wide-sqbox")
		const maxW, maxH = 100.0, 100.0
		w, h := scaledLogoSize(info, maxW, maxH)
		if !approxEqual(w, 100) || !approxEqual(h, 50) {
			t.Fatalf("scaledLogoSize(8x4, 100, 100) = (%v, %v), want (100, 50)", w, h)
		}
		if w > maxW+eps || h > maxH+eps {
			t.Fatalf("output (%v, %v) exceeds box (%v, %v)", w, h, maxW, maxH)
		}
	})

	t.Run("tall_image_in_square_box_height_binds", func(t *testing.T) {
		// 4x8 aspect 1:2 in a 1:1 box: height binds, output is (50, 100).
		info := registerLogoTestImage(t, 4, 8, "tall-sqbox")
		const maxW, maxH = 100.0, 100.0
		w, h := scaledLogoSize(info, maxW, maxH)
		if !approxEqual(w, 50) || !approxEqual(h, 100) {
			t.Fatalf("scaledLogoSize(4x8, 100, 100) = (%v, %v), want (50, 100)", w, h)
		}
		if w > maxW+eps || h > maxH+eps {
			t.Fatalf("output (%v, %v) exceeds box (%v, %v)", w, h, maxW, maxH)
		}
	})

	t.Run("wide_image_in_matched_aspect_box_fills_both", func(t *testing.T) {
		// 8x4 aspect 2:1 in a 100x50 box (also 2:1): both axes bind at once.
		info := registerLogoTestImage(t, 8, 4, "wide-matched")
		const maxW, maxH = 100.0, 50.0
		w, h := scaledLogoSize(info, maxW, maxH)
		if !approxEqual(w, 100) || !approxEqual(h, 50) {
			t.Fatalf("scaledLogoSize(8x4, 100, 50) = (%v, %v), want (100, 50)", w, h)
		}
	})

	t.Run("tall_image_in_matched_aspect_box_fills_both", func(t *testing.T) {
		// 4x8 aspect 1:2 in a 50x100 box (also 1:2): both axes bind at once.
		info := registerLogoTestImage(t, 4, 8, "tall-matched")
		const maxW, maxH = 50.0, 100.0
		w, h := scaledLogoSize(info, maxW, maxH)
		if !approxEqual(w, 50) || !approxEqual(h, 100) {
			t.Fatalf("scaledLogoSize(4x8, 50, 100) = (%v, %v), want (50, 100)", w, h)
		}
	})

	t.Run("image_smaller_than_box_uses_uniform_scale", func(t *testing.T) {
		// Verify aspect ratio is preserved across a non-square box where the
		// image is upscaling rather than downscaling. 6x2 aspect 3:1 in a
		// 300x100 box (also 3:1): both axes bind.
		info := registerLogoTestImage(t, 6, 2, "upscale")
		const maxW, maxH = 300.0, 100.0
		w, h := scaledLogoSize(info, maxW, maxH)
		if !approxEqual(w, 300) || !approxEqual(h, 100) {
			t.Fatalf("scaledLogoSize(6x2, 300, 100) = (%v, %v), want (300, 100)", w, h)
		}
		if w > maxW+eps || h > maxH+eps {
			t.Fatalf("output (%v, %v) exceeds box (%v, %v)", w, h, maxW, maxH)
		}
	})
}
