package bind

import (
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Common 包含操作系统相关信息
type Common struct {
}

// GetOS 获取当前操作系统类型
func (o *Common) GetOS() string {
	return runtime.GOOS
}

// GetArch 获取当前系统架构
func (o *Common) GetArch() string {
	return runtime.GOARCH
}

// FileExists 检查文件是否存在
func (o *Common) FileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// ReadFile 读取文件内容
func (o *Common) ReadFile(path string) (string, error) {
	log.Println("ReadFile", path)
	if strings.HasPrefix(path, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(homeDir, path[2:])
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// WriteFile 写入文件内容
func (o *Common) WriteFile(path string, content string) error {
	log.Println("WriteFile", path)
	if strings.HasPrefix(path, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		path = filepath.Join(homeDir, path[2:])
	}
	return os.WriteFile(path, []byte(content), 0644)
}
