package tools

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// maxImageBytes is above Discord's largest asset limit (banners/stickers).
const maxImageBytes = 10 << 20

var imageHTTP = &http.Client{Timeout: 30 * time.Second}

// toDataURI turns a local path, an http(s) URL or an existing data URI into a
// data URI, which is what Discord expects for icons, avatars, emojis, etc.
func toDataURI(ctx context.Context, src string) (string, error) {
	src = strings.TrimSpace(src)
	switch {
	case src == "":
		return "", errors.New("empty image source")
	case strings.HasPrefix(src, "data:"):
		return src, nil
	case strings.HasPrefix(src, "http://"), strings.HasPrefix(src, "https://"):
		return fetchImage(ctx, src)
	default:
		return readImage(src)
	}
}

func fetchImage(ctx context.Context, u string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", fmt.Errorf("image url: %w", err)
	}
	resp, err := imageHTTP.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch image: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch image: HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxImageBytes+1))
	if err != nil {
		return "", fmt.Errorf("fetch image: %w", err)
	}
	return encode(data, resp.Header.Get("Content-Type"))
}

func readImage(path string) (string, error) {
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			path = filepath.Join(home, path[2:])
		}
	}
	f, err := os.Open(filepath.Clean(path))
	if err != nil {
		return "", fmt.Errorf("read image: %w", err)
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, maxImageBytes+1))
	if err != nil {
		return "", fmt.Errorf("read image: %w", err)
	}
	return encode(data, "")
}

func encode(data []byte, contentType string) (string, error) {
	if len(data) > maxImageBytes {
		return "", fmt.Errorf("image larger than %d MiB", maxImageBytes>>20)
	}
	ct, _, _ := strings.Cut(contentType, ";")
	if !isMedia(ct) {
		ct = http.DetectContentType(data)
	}
	if ct == "application/ogg" {
		ct = "audio/ogg"
	}
	if !isMedia(ct) {
		return "", fmt.Errorf("not an image or audio file (detected %s)", ct)
	}
	return "data:" + ct + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}

func isMedia(ct string) bool {
	return strings.HasPrefix(ct, "image/") || strings.HasPrefix(ct, "audio/") || ct == "application/ogg"
}
