package web

import (
	"fmt"

	"Puff/storage"
)

// addDomainToConfig 添加域名到配置文件
func (s *Server) addDomainToConfig(domain string) error {
	if err := storage.AddDomain(domain, true, true); err != nil {
		return fmt.Errorf("domain already exists or写入失败: %w", err)
	}
	return nil
}

// removeDomainFromConfig 从配置文件删除域名
func (s *Server) removeDomainFromConfig(domain string) error {
	if err := storage.RemoveDomain(domain); err != nil {
		return fmt.Errorf("domain not found or删除失败: %w", err)
	}
	return nil
}
