package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const codesDir = "./codes"

// ListCodeFiles - GET /v1/codes/files
// Возвращает список сохранённых файлов с кодами
func ListCodeFiles(c *gin.Context) {
	entries, err := os.ReadDir(codesDir)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusOK, gin.H{"files": []interface{}{}})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "не удалось прочитать папку с кодами"})
		return
	}

	type FileInfo struct {
		Name      string    `json:"name"`
		Size      int64     `json:"size"`
		CreatedAt time.Time `json:"created_at"`
	}

	files := []FileInfo{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".txt") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, FileInfo{
			Name:      e.Name(),
			Size:      info.Size(),
			CreatedAt: info.ModTime(),
		})
	}

	c.JSON(http.StatusOK, gin.H{"files": files})
}

// DownloadCodeFile - GET /v1/codes/files/:filename
// Позволяет скачать файл с кодами
func DownloadCodeFile(c *gin.Context) {
	filename := c.Param("filename")

	// Защита от path traversal
	if strings.Contains(filename, "/") || strings.Contains(filename, "..") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "недопустимое имя файла"})
		return
	}

	fullPath := filepath.Join(codesDir, filename)
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "файл не найден"})
		return
	}

	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.File(fullPath)
}
