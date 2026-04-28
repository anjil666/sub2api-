package handler

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

func (h *GatewayHandler) ImageProxy(c *gin.Context) {
	url := c.Query("url")
	if url == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "url parameter required"})
		return
	}
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "invalid url"})
		return
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to fetch image"})
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusBadGateway, gin.H{"error": "upstream returned " + resp.Status})
		return
	}
	fn := c.Query("fn")
	if fn == "" {
		fn = "image.png"
	}
	imgData, err := io.ReadAll(io.LimitReader(resp.Body, 50<<20))
	if err != nil || len(imgData) == 0 {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to read image"})
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", fn))
	c.Header("Cache-Control", "no-cache")
	c.Data(http.StatusOK, "application/octet-stream", imgData)
}

type dlEntry struct {
	data    []byte
	name    string
	created time.Time
}

var (
	dlStore   sync.Map
	dlCleanup sync.Once
)

func cleanupDLStore() {
	go func() {
		for {
			time.Sleep(60 * time.Second)
			now := time.Now()
			dlStore.Range(func(key, value any) bool {
				if e, ok := value.(*dlEntry); ok && now.Sub(e.created) > 5*time.Minute {
					dlStore.Delete(key)
				}
				return true
			})
		}
	}()
}

func (h *GatewayHandler) ImageDownloadPrepare(c *gin.Context) {
	dlCleanup.Do(cleanupDLStore)
	data := c.PostForm("data")
	fn := c.PostForm("fn")
	if fn == "" {
		fn = "image.png"
	}
	if data == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "data required"})
		return
	}
	imgData, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid base64"})
		return
	}
	b := make([]byte, 16)
	rand.Read(b)
	id := hex.EncodeToString(b)
	dlStore.Store(id, &dlEntry{data: imgData, name: fn, created: time.Now()})
	token := c.Query("token")
	c.Redirect(http.StatusFound, fmt.Sprintf("/v1/user/image-download/%s?token=%s", id, token))
}

func (h *GatewayHandler) ImageDownloadGet(c *gin.Context) {
	id := c.Param("id")
	val, ok := dlStore.LoadAndDelete(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "download expired"})
		return
	}
	e := val.(*dlEntry)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", e.name))
	c.Header("Cache-Control", "no-cache")
	c.Data(http.StatusOK, "application/octet-stream", e.data)
}
