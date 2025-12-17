package core

import (
	"fmt"
	"strings"
	"time"

	"Puff/config"
	"Puff/logger"
)

// DomainChecker 域名检查器
type DomainChecker struct {
	whoisClient *WhoisClient
	rdapClient  *RDAPClient
	config      *config.Config
}

// NewDomainChecker 创建新的域名检查器
func NewDomainChecker(cfg *config.Config) *DomainChecker {
	return &DomainChecker{
		whoisClient: NewWhoisClient(cfg.Monitor.Timeout),
		rdapClient:  NewRDAPClient(cfg.Monitor.Timeout),
		config:      cfg,
	}
}

// UpdateConfig 更新配置（用于热重载）
func (d *DomainChecker) UpdateConfig(cfg *config.Config) {
	d.config = cfg
	d.whoisClient.timeout = cfg.Monitor.Timeout
	d.rdapClient.httpClient.Timeout = cfg.Monitor.Timeout
}

// CheckDomain 检查单个域名
func (d *DomainChecker) CheckDomain(domain string) *DomainInfo {
	domain = strings.ToLower(strings.TrimSpace(domain))

	// 获取TLD（最长后缀匹配）
	tld := config.FindBestTLD(domain)
	if tld == "" {
		return &DomainInfo{
			Name:         domain,
			Status:       StatusError,
			ErrorMessage: "不支持的TLD",
			LastChecked:  time.Now(),
		}
	}

	// 首先尝试RDAP查询
	if rdapInfo := d.tryRDAPQuery(domain, tld); rdapInfo != nil && rdapInfo.Status != StatusError {
		return rdapInfo
	}

	// RDAP失败，尝试WHOIS查询
	if whoisInfo := d.tryWhoisQuery(domain, tld); whoisInfo != nil {
		return whoisInfo
	}

	// 都失败了
	return &DomainInfo{
		Name:         domain,
		Status:       StatusError,
		ErrorMessage: "WHOIS和RDAP查询都失败",
		LastChecked:  time.Now(),
	}
}

// ValidateDomain 验证域名格式
func (dc *DomainChecker) ValidateDomain(domain string) error {
	domain = strings.TrimSpace(domain)

	if domain == "" {
		return fmt.Errorf("域名不能为空")
	}

	// 基本长度检查
	if len(domain) > 253 {
		return fmt.Errorf("域名长度不能超过253个字符")
	}

	// 检查是否包含无效字符
	for _, char := range domain {
		if !((char >= 'a' && char <= 'z') ||
			(char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') ||
			char == '.' || char == '-') {
			return fmt.Errorf("域名包含无效字符: %c", char)
		}
	}

	// 分割域名部分
	parts := strings.Split(domain, ".")
	if len(parts) < 2 {
		return fmt.Errorf("域名必须包含至少一个点")
	}

	// 检查每个部分
	for i, part := range parts {
		if part == "" {
			return fmt.Errorf("域名部分不能为空")
		}

		if len(part) > 63 {
			return fmt.Errorf("域名部分长度不能超过63个字符")
		}

		// 不能以连字符开始或结束
		if strings.HasPrefix(part, "-") || strings.HasSuffix(part, "-") {
			return fmt.Errorf("域名部分不能以连字符开始或结束: %s", part)
		}

		// 最后一部分（TLD）不能全是数字
		if i == len(parts)-1 {
			allDigits := true
			for _, char := range part {
				if char < '0' || char > '9' {
					allDigits = false
					break
				}
			}
			if allDigits {
				return fmt.Errorf("顶级域名不能全是数字")
			}
		}
	}

	return nil
}

// tryRDAPQuery 尝试RDAP查询
func (d *DomainChecker) tryRDAPQuery(domain, tld string) *DomainInfo {
	server, exists := config.GetRDAPServerByTLD(domain)
	if !exists {
		return &DomainInfo{
			Name:         domain,
			Status:       StatusError,
			ErrorMessage: fmt.Sprintf("不支持的TLD或缺少RDAP服务器: %s", tld),
			LastChecked:  time.Now(),
		}
	}

	// 使用新的QueryRDAPWithRaw方法同时获取解析后的数据和原始JSON
	rdapResp, rawJSON, err := d.rdapClient.QueryRDAPWithRaw(domain, server.Server)
	if err != nil {
		return &DomainInfo{
			Name:         domain,
			Status:       StatusError,
			ErrorMessage: fmt.Sprintf("RDAP查询失败: %v", err),
			LastChecked:  time.Now(),
		}
	}

	// 解析RDAP响应并传入原始JSON数据
	return d.rdapClient.ParseRDAPResponse(domain, rdapResp, rawJSON)
}

// tryWhoisQuery 尝试WHOIS查询（不带重试，重试由外层CheckDomainsWithCallback处理）
func (d *DomainChecker) tryWhoisQuery(domain, tld string) *DomainInfo {
	server, exists := config.GetWhoisServerByTLD(domain)
	if !exists {
		logger.Debug("Domain checker: no WHOIS server found for domain=%s tld=%s", domain, tld)
		return &DomainInfo{
			Name:         domain,
			Status:       StatusError,
			ErrorMessage: fmt.Sprintf("不支持的TLD: %s (WHOIS)", tld),
			LastChecked:  time.Now(),
		}
	}

	// 单次WHOIS查询，不在此处重试
	response, err := d.whoisClient.QueryWhois(domain, server.Server, server.Port)
	if err != nil {
		logger.Debug("Domain checker: WHOIS query failed for domain=%s err=%v", domain, err)
		return &DomainInfo{
			Name:         domain,
			Status:       StatusError,
			ErrorMessage: fmt.Sprintf("WHOIS连接失败: %v", err),
			LastChecked:  time.Now(),
		}
	}

	// WHOIS查询成功
	result := d.whoisClient.ParseWhoisResponse(domain, response)
	// 保存原始WHOIS数据
	result.WhoisRaw = response
	logger.Debug("Domain checker: WHOIS parsed result for domain=%s status=%s", domain, result.Status)

	// 如果结果是unknown且响应很短，可能是网络问题
	if result.Status == StatusUnknown && len(response) < 50 {
		logger.Debug("Domain checker: WHOIS got unknown status with short response for domain=%s", domain)
		return &DomainInfo{
			Name:         domain,
			Status:       StatusError,
			ErrorMessage: "WHOIS响应过短，可能是网络问题",
			LastChecked:  time.Now(),
		}
	}

	return result
}

// GetSupportedTLDs 获取支持的TLD列表
func (d *DomainChecker) GetSupportedTLDs() []string {
	return config.GetSupportedTLDs()
}
