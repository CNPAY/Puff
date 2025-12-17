package config

import (
	"embed"
)

//go:embed detection_patterns.json servers.json
var configFiles embed.FS

// GetEmbeddedFile 获取嵌入的配置文件内容
func GetEmbeddedFile(filename string) ([]byte, error) {
	return configFiles.ReadFile(filename)
}
