package core

import (
	"Puff/logger"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// RDAPClient RDAP查询客户端
type RDAPClient struct {
	httpClient *http.Client
}

// NewRDAPClient 创建新的RDAP客户端
func NewRDAPClient(timeout time.Duration) *RDAPClient {
	return &RDAPClient{
		httpClient: GetProxyHTTPClient(timeout),
	}
}

// RDAPResponse RDAP响应结构
type RDAPResponse struct {
	ObjectClassName string           `json:"objectClassName"`
	Handle          string           `json:"handle"`
	LDHName         string           `json:"ldhName"`
	Status          []string         `json:"status"`
	Entities        []RDAPEntity     `json:"entities"`
	Events          []RDAPEvent      `json:"events"`
	NameServers     []RDAPNameServer `json:"nameservers"`
	ErrorCode       int              `json:"errorCode,omitempty"`
	Title           string           `json:"title,omitempty"`
	Description     []string         `json:"description,omitempty"`
}

// RDAPEntity RDAP实体结构
type RDAPEntity struct {
	ObjectClassName string        `json:"objectClassName"`
	Handle          string        `json:"handle"`
	Roles           []string      `json:"roles"`
	VCardArray      []interface{} `json:"vcardArray,omitempty"`
}

// RDAPEvent RDAP事件结构
type RDAPEvent struct {
	EventAction string    `json:"eventAction"`
	EventDate   time.Time `json:"eventDate"`
}

// RDAPNameServer RDAP名称服务器结构
type RDAPNameServer struct {
	ObjectClassName string `json:"objectClassName"`
	LDHName         string `json:"ldhName"`
	IPAddresses     struct {
		V4 []string `json:"v4,omitempty"`
		V6 []string `json:"v6,omitempty"`
	} `json:"ipAddresses,omitempty"`
}

// QueryRDAP 执行RDAP查询
func (r *RDAPClient) QueryRDAP(domain, serverURL string) (*RDAPResponse, error) {
	if strings.TrimSpace(serverURL) == "" {
		return nil, fmt.Errorf("RDAP服务器地址为空")
	}

	// 构建查询URL
	url := strings.TrimSuffix(serverURL, "/") + "/domain/" + domain

	// 创建HTTP请求
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Printf("RDAP build request failed domain=%s url=%s err=%v", domain, url, err)
		return nil, fmt.Errorf("创建HTTP请求失败: %v", err)
	}

	// 设置请求头
	req.Header.Set("Accept", "application/rdap+json")
	// 使用标准浏览器User-Agent，某些RDAP服务器（如ch/li）会拒绝非浏览器请求
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	// 执行请求
	resp, err := r.httpClient.Do(req)
	if err != nil {
		log.Printf("RDAP request failed domain=%s url=%s err=%v", domain, url, err)
		return nil, fmt.Errorf("执行HTTP请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应体失败: %v", err)
	}

	// 检查HTTP状态码
	if resp.StatusCode == 404 {
		// 404通常表示域名不存在
		// RDAP 404 - 域名不存在，这是正常情况
		return &RDAPResponse{
			ErrorCode:   404,
			Title:       "Not Found",
			Description: []string{"Domain not found"},
		}, nil
	}

	if resp.StatusCode == 429 {
		// 429 Too Many Requests - 限流错误，返回特殊错误以便重试
		return nil, fmt.Errorf("RDAP服务器限流(429)，请稍后重试")
	}

	if resp.StatusCode != 200 {
		log.Printf("RDAP non-200 domain=%s url=%s status=%d", domain, url, resp.StatusCode)
		return nil, fmt.Errorf("HTTP请求失败，状态码: %d", resp.StatusCode)
	}

	// 解析JSON响应
	var rdapResp RDAPResponse
	if err := json.Unmarshal(body, &rdapResp); err != nil {
		return nil, fmt.Errorf("解析JSON响应失败: %v", err)
	}

	return &rdapResp, nil
}

// QueryRDAPWithRaw 执行RDAP查询并返回原始JSON数据
func (r *RDAPClient) QueryRDAPWithRaw(domain, serverURL string) (*RDAPResponse, string, error) {
	if strings.TrimSpace(serverURL) == "" {
		return nil, "", fmt.Errorf("RDAP服务器地址为空")
	}

	// 构建查询URL
	url := strings.TrimSuffix(serverURL, "/") + "/domain/" + domain

	// 创建HTTP请求
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Printf("RDAP build request failed domain=%s url=%s err=%v", domain, url, err)
		return nil, "", fmt.Errorf("创建HTTP请求失败: %v", err)
	}

	// 设置请求头
	req.Header.Set("Accept", "application/rdap+json")
	// 使用标准浏览器User-Agent，某些RDAP服务器（如ch/li）会拒绝非浏览器请求
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	// 执行请求
	resp, err := r.httpClient.Do(req)
	if err != nil {
		log.Printf("RDAP request failed domain=%s url=%s err=%v", domain, url, err)
		return nil, "", fmt.Errorf("执行HTTP请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("读取响应体失败: %v", err)
	}

	rawJSON := string(body)

	// 检查HTTP状态码
	if resp.StatusCode == 404 {
		// 404通常表示域名不存在
		return &RDAPResponse{
			ErrorCode:   404,
			Title:       "Not Found",
			Description: []string{"Domain not found"},
		}, rawJSON, nil
	}

	if resp.StatusCode == 429 {
		// 429 Too Many Requests - 限流错误，返回特殊错误以便重试
		return nil, "", fmt.Errorf("RDAP服务器限流(429)，请稍后重试")
	}

	if resp.StatusCode != 200 {
		log.Printf("RDAP non-200 domain=%s url=%s status=%d", domain, url, resp.StatusCode)
		return nil, rawJSON, fmt.Errorf("HTTP请求失败，状态码: %d", resp.StatusCode)
	}

	// 解析JSON响应
	var rdapResp RDAPResponse
	if err := json.Unmarshal(body, &rdapResp); err != nil {
		return nil, rawJSON, fmt.Errorf("解析JSON响应失败: %v", err)
	}

	return &rdapResp, rawJSON, nil
}

// ParseRDAPResponse 解析RDAP响应
func (r *RDAPClient) ParseRDAPResponse(domain string, rdapResp *RDAPResponse, rawJSON string) *DomainInfo {
	info := &DomainInfo{
		Name:        domain,
		LastChecked: time.Now(),
		QueryMethod: "rdap",
		WhoisRaw:    rawJSON, // 保存原始RDAP JSON数据
	}

	// 检查是否为错误响应
	if rdapResp.ErrorCode == 404 {
		info.Status = StatusAvailable
		return info
	}

	// 解析域名状态
	info.Status = r.parseRDAPStatus(rdapResp.Status)

	// 解析注册商
	info.Registrar = r.parseRDAPRegistrar(rdapResp.Entities)

	// 解析事件日期
	r.parseRDAPEvents(rdapResp.Events, info)

	// 解析名称服务器
	info.NameServers = r.parseRDAPNameServers(rdapResp.NameServers)

	// 如果状态仍未知，尝试根据描述/标题判断可注册
	if info.Status == StatusUnknown {
		desc := strings.ToLower(strings.Join(rdapResp.Description, " "))
		title := strings.ToLower(rdapResp.Title)
		if strings.Contains(title, "not found") || strings.Contains(desc, "not found") || strings.Contains(desc, "no match") || strings.Contains(desc, "domain not found") {
			info.Status = StatusAvailable
		}
	}

	// 额外校验：如果判定为可注册，但存在关键注册信息，则认为是误报
	if info.Status == StatusAvailable {
		hasValidRegistrar := info.Registrar != "" && !strings.Contains(info.Registrar, "不支持")
		hasExpiryDate := info.ExpiryDate != nil
		hasCreatedDate := info.CreatedDate != nil
		hasEvents := len(rdapResp.Events) > 0

		if hasValidRegistrar || hasExpiryDate || hasCreatedDate || hasEvents {
			logger.Warn("域名 %s (RDAP) 被误判为可注册，检测到注册信息，修正为已注册", domain)
			info.Status = StatusRegistered
		}
	}

	// 最终状态校验：只有在状态未知且有注册信息时，才设置为已注册
	// 不要覆盖已经正确检测到的特殊状态（如宽限期、赎回期、待删除等）
	if info.Status == StatusUnknown {
		hasValidRegistrar := info.Registrar != "" && !strings.Contains(info.Registrar, "不支持")
		hasNameServers := len(info.NameServers) > 0
		hasExpiryDate := info.ExpiryDate != nil
		hasCreatedDate := info.CreatedDate != nil
		hasEvents := len(rdapResp.Events) > 0

		// 如果有任何实际的注册信息，则认定为已注册
		if hasValidRegistrar || hasNameServers || hasExpiryDate || hasCreatedDate || hasEvents {
			info.Status = StatusRegistered
		} else {
			// 没有任何注册信息且状态未知时，认定为可注册
			info.Status = StatusAvailable
		}
	}

	// 仍未知则输出原始 RDAP 信息便于排查
	if info.Status == StatusUnknown {
		log.Printf("RDAP raw unknown %s: status=%v title=%s", domain, rdapResp.Status, rdapResp.Title)
	}

	return info
}

// parseRDAPStatus 解析RDAP状态
func (r *RDAPClient) parseRDAPStatus(statuses []string) DomainStatus {
	if len(statuses) == 0 {
		return StatusUnknown
	}

	// 将状态转换为小写进行匹配
	statusMap := make(map[string]bool)
	for _, status := range statuses {
		statusMap[strings.ToLower(status)] = true
	}

	// 赎回期
	if statusMap["redemption period"] || statusMap["redemptionperiod"] {
		return StatusRedemption
	}

	// 待删除
	if statusMap["pending delete"] || statusMap["pendingdelete"] {
		return StatusPendingDelete
	}

	// 宽限/续费
	if statusMap["renew period"] || statusMap["auto renew period"] || statusMap["expired"] {
		return StatusGrace
	}

	// 其余视为已注册
	return StatusRegistered
}

// parseRDAPRegistrar 解析注册商信息
func (r *RDAPClient) parseRDAPRegistrar(entities []RDAPEntity) string {
	for _, entity := range entities {
		// 寻找registrar角色
		for _, role := range entity.Roles {
			if strings.ToLower(role) == "registrar" {
				// 尝试从vCard中提取组织名称
				if orgName := r.extractOrgFromVCard(entity.VCardArray); orgName != "" {
					return orgName
				}
				// 如果没有vCard信息，返回handle
				return entity.Handle
			}
		}
	}
	return ""
}

// extractOrgFromVCard 从vCard中提取组织名称
func (r *RDAPClient) extractOrgFromVCard(vcard []interface{}) string {
	if len(vcard) < 2 {
		return ""
	}

	// vCard数组的第二个元素包含属性
	if properties, ok := vcard[1].([]interface{}); ok {
		for _, prop := range properties {
			if propArray, ok := prop.([]interface{}); ok && len(propArray) >= 4 {
				// 检查是否为组织属性
				if propName, ok := propArray[0].(string); ok && strings.ToLower(propName) == "org" {
					if propValue, ok := propArray[3].(string); ok {
						return propValue
					}
				}
				// 检查fn（全名）属性
				if propName, ok := propArray[0].(string); ok && strings.ToLower(propName) == "fn" {
					if propValue, ok := propArray[3].(string); ok {
						return propValue
					}
				}
			}
		}
	}

	return ""
}

// parseRDAPEvents 解析RDAP事件
func (r *RDAPClient) parseRDAPEvents(events []RDAPEvent, info *DomainInfo) {
	for _, event := range events {
		switch strings.ToLower(event.EventAction) {
		case "registration":
			info.CreatedDate = &event.EventDate
		case "expiration":
			info.ExpiryDate = &event.EventDate
		case "soft expiration":
			info.ExpiryDate = &event.EventDate
		case "last changed", "last update of rdap database":
			info.UpdatedDate = &event.EventDate
		}
	}
}

// parseRDAPNameServers 解析RDAP名称服务器
func (r *RDAPClient) parseRDAPNameServers(nameservers []RDAPNameServer) []string {
	var result []string
	seen := make(map[string]bool)

	for _, ns := range nameservers {
		if ns.LDHName != "" {
			lowerName := strings.ToLower(ns.LDHName)
			if !seen[lowerName] {
				result = append(result, lowerName)
				seen[lowerName] = true
			}
		}
	}

	return result
}
