package config

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
)

// WhoisServer WHOIS服务器配置
type WhoisServer struct {
	Server string `json:"server"` // 服务器地址
	Port   int    `json:"port"`   // 端口
}

// RDAPServer RDAP服务器配置
type RDAPServer struct {
	Server string `json:"server"` // 服务器地址
}

// TLDServers TLD服务器配置
type TLDServers struct {
	Whois WhoisServer `json:"whois"`
	RDAP  RDAPServer  `json:"rdap"`
}

// DetectionPatterns 检测模式配置
type DetectionPatterns struct {
	AvailablePatterns     []string `json:"available_patterns"`
	RedemptionPatterns    []string `json:"redemption_patterns"`
	PendingDeletePatterns []string `json:"pending_delete_patterns"`
	ExpiredPatterns       []string `json:"expired_patterns"`
	HoldPatterns          []string `json:"hold_patterns"`
	TransferLockPatterns  []string `json:"transfer_lock_patterns"`
	RegisteredPatterns    []string `json:"registered_patterns"`
	GracePatterns         []string `json:"grace_patterns"`
}

var (
	serversConfig  map[string]TLDServers
	patternsConfig DetectionPatterns
	configMutex    sync.RWMutex
	configLoaded   bool
)

// LoadServerConfigs 加载服务器配置（从嵌入的文件读取）
func LoadServerConfigs() error {
	configMutex.Lock()
	defer configMutex.Unlock()

	// 读取嵌入的 servers.json 文件
	sData, err := GetEmbeddedFile("servers.json")
	if err != nil {
		return fmt.Errorf("读取 servers.json 失败: %v", err)
	}
	if err := json.Unmarshal(sData, &serversConfig); err != nil {
		return fmt.Errorf("解析 servers.json 失败: %v", err)
	}

	// 读取嵌入的 detection_patterns.json 文件
	pData, err := GetEmbeddedFile("detection_patterns.json")
	if err != nil {
		return fmt.Errorf("读取 detection_patterns.json 失败: %v", err)
	}
	if err := json.Unmarshal(pData, &patternsConfig); err != nil {
		return fmt.Errorf("解析 detection_patterns.json 失败: %v", err)
	}

	if len(patternsConfig.AvailablePatterns) == 0 {
		return fmt.Errorf("detection_patterns.json 中 available_patterns 为空，配置无效")
	}

	configLoaded = true
	return nil
}

// ReloadServerConfigs 重新加载服务器配置（用于TLD更新后刷新）
func ReloadServerConfigs() error {
	return LoadServerConfigs()
}

// GetWhoisServerByTLD 根据TLD获取WHOIS服务器
func GetWhoisServerByTLD(domain string) (WhoisServer, bool) {
	if !configLoaded {
		if err := LoadServerConfigs(); err != nil {
			log.Printf("Config: failed to load server configs: %v", err)
			return WhoisServer{}, false
		}
	}

	configMutex.RLock()
	defer configMutex.RUnlock()

	key := findBestTLD(domain)
	if key == "" {
		log.Printf("Config: no TLD match found for domain=%s", domain)
		return WhoisServer{}, false
	}

	server := serversConfig[key].Whois
	// WHOIS服务器已找到

	if server.Server == "" {
		log.Printf("Config: empty WHOIS server for domain=%s tld=%s", domain, key)
		return WhoisServer{}, false
	}

	return server, true
}

// GetRDAPServerByTLD 根据TLD获取RDAP服务器
func GetRDAPServerByTLD(domain string) (RDAPServer, bool) {
	if !configLoaded {
		if err := LoadServerConfigs(); err != nil {
			return RDAPServer{}, false
		}
	}

	configMutex.RLock()
	defer configMutex.RUnlock()

	key := findBestTLD(domain)
	if key == "" {
		return RDAPServer{}, false
	}

	srv := serversConfig[key].RDAP
	if strings.TrimSpace(srv.Server) == "" {
		return RDAPServer{}, false
	}
	return srv, true
}

// GetDetectionPatterns 获取检测模式
func GetDetectionPatterns() DetectionPatterns {
	if !configLoaded {
		if err := LoadServerConfigs(); err != nil {
			log.Printf("Config: failed to load detection patterns: %v", err)
		}
	}

	configMutex.RLock()
	defer configMutex.RUnlock()

	// 检测模式已加载

	return patternsConfig
}

// GetSupportedTLDs 获取支持的TLD列表
func GetSupportedTLDs() []string {
	if !configLoaded {
		if err := LoadServerConfigs(); err != nil {
			return []string{}
		}
	}

	configMutex.RLock()
	defer configMutex.RUnlock()

	tlds := make([]string, 0, len(serversConfig))
	for tld := range serversConfig {
		tlds = append(tlds, tld)
	}

	return tlds
}

// FindBestTLD 对外暴露最长匹配
func FindBestTLD(domain string) string {
	if !configLoaded {
		if err := LoadServerConfigs(); err != nil {
			return ""
		}
	}
	configMutex.RLock()
	defer configMutex.RUnlock()
	return findBestTLD(domain)
}

// findBestTLD 在配置中匹配最长后缀
func findBestTLD(domain string) string {
	domain = strings.ToLower(domain)
	best := ""
	for tld := range serversConfig {
		lt := strings.ToLower(tld)
		if domain == lt || strings.HasSuffix(domain, "."+lt) {
			if len(lt) > len(best) {
				best = lt
			}
		}
	}
	return best
}
