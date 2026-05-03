package service

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"math"
	"strconv"
	"strings"

	_ "image/jpeg"

	"github.com/tidwall/gjson"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

func NeedsUpscale(size string) bool {
	w, h := ParseSizeDimensions(size)
	return w > 1024 || h > 1024
}

func ParseSizeDimensions(size string) (int, int) {
	parts := strings.SplitN(strings.ToLower(strings.TrimSpace(size)), "x", 2)
	if len(parts) != 2 {
		return 0, 0
	}
	w, err1 := strconv.Atoi(parts[0])
	h, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil || w <= 0 || h <= 0 {
		return 0, 0
	}
	return w, h
}

func UpscaleImageBase64(b64 string, targetW, targetH int) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return b64, fmt.Errorf("decode base64: %w", err)
	}

	src, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return b64, fmt.Errorf("decode image: %w", err)
	}

	bounds := src.Bounds()
	srcW, srcH := bounds.Dx(), bounds.Dy()
	if srcW >= targetW && srcH >= targetH {
		return b64, nil
	}

	dst := image.NewRGBA(image.Rect(0, 0, targetW, targetH))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, bounds, draw.Over, nil)

	sharpen(dst, 1.2, 1)

	var buf bytes.Buffer
	enc := &png.Encoder{CompressionLevel: png.BestSpeed}
	if err := enc.Encode(&buf, dst); err != nil {
		return b64, fmt.Errorf("encode png: %w", err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

func UpscaleResponseImages(respBody []byte, targetW, targetH int) []byte {
	dataArr := gjson.GetBytes(respBody, "data")
	if !dataArr.IsArray() {
		return respBody
	}
	hasB64 := false
	for _, item := range dataArr.Array() {
		if item.Get("b64_json").Exists() {
			hasB64 = true
			break
		}
	}
	if !hasB64 {
		return respBody
	}

	var parsed map[string]any
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return respBody
	}
	dataSlice, ok := parsed["data"].([]any)
	if !ok {
		return respBody
	}
	for _, item := range dataSlice {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		b64Val, ok := m["b64_json"].(string)
		if !ok || b64Val == "" {
			continue
		}
		upscaled, err := UpscaleImageBase64(b64Val, targetW, targetH)
		if err != nil {
			continue
		}
		m["b64_json"] = upscaled
	}
	out, err := json.Marshal(parsed)
	if err != nil {
		return respBody
	}
	return out
}

func sharpen(img *image.RGBA, amount float64, radius int) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	kernel := makeGaussianKernel(radius)
	ksize := len(kernel)
	khalf := ksize / 2

	tmp := make([]uint8, len(img.Pix))
	copy(tmp, img.Pix)

	blurred := make([]uint8, len(img.Pix))

	// horizontal pass
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var r, g, b0 float64
			for k := 0; k < ksize; k++ {
				sx := x + k - khalf
				if sx < 0 {
					sx = 0
				} else if sx >= w {
					sx = w - 1
				}
				off := (y*w + sx) * 4
				wt := kernel[k]
				r += float64(tmp[off]) * wt
				g += float64(tmp[off+1]) * wt
				b0 += float64(tmp[off+2]) * wt
			}
			off := (y*w + x) * 4
			blurred[off] = uint8(math.Round(r))
			blurred[off+1] = uint8(math.Round(g))
			blurred[off+2] = uint8(math.Round(b0))
			blurred[off+3] = tmp[off+3]
		}
	}

	// vertical pass
	copy(tmp, blurred)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var r, g, b0 float64
			for k := 0; k < ksize; k++ {
				sy := y + k - khalf
				if sy < 0 {
					sy = 0
				} else if sy >= h {
					sy = h - 1
				}
				off := (sy*w + x) * 4
				wt := kernel[k]
				r += float64(tmp[off]) * wt
				g += float64(tmp[off+1]) * wt
				b0 += float64(tmp[off+2]) * wt
			}
			off := (y*w + x) * 4
			blurred[off] = uint8(math.Round(r))
			blurred[off+1] = uint8(math.Round(g))
			blurred[off+2] = uint8(math.Round(b0))
			blurred[off+3] = tmp[off+3]
		}
	}

	// unsharp mask: result = original + amount * (original - blurred)
	for i := 0; i < len(img.Pix); i += 4 {
		for c := 0; c < 3; c++ {
			orig := float64(img.Pix[i+c])
			blur := float64(blurred[i+c])
			v := orig + amount*(orig-blur)
			img.Pix[i+c] = clampU8(v)
		}
	}
}

func makeGaussianKernel(radius int) []float64 {
	size := radius*2 + 1
	sigma := float64(radius) * 0.5
	if sigma < 0.5 {
		sigma = 0.5
	}
	k := make([]float64, size)
	var sum float64
	for i := 0; i < size; i++ {
		x := float64(i - radius)
		k[i] = math.Exp(-(x * x) / (2 * sigma * sigma))
		sum += k[i]
	}
	for i := range k {
		k[i] /= sum
	}
	return k
}

func clampU8(v float64) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(math.Round(v))
}
