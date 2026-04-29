package service

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
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
