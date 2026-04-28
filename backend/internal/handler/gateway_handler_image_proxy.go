package handler

import (
	"io"
	"net/http"
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
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	fn := c.Query("fn")
	if fn == "" {
		fn = "image.png"
	}
	c.Header("Content-Disposition", "attachment; filename=\""+fn+"\"")
	c.Header("Cache-Control", "no-cache")
	c.Status(http.StatusOK)
	c.Header("Content-Type", contentType)
	io.Copy(c.Writer, io.LimitReader(resp.Body, 20<<20))
}
