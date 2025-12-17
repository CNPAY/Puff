package core

import (
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"

	"Puff/config"
	"Puff/logger"
)

// WhoisClient WHOIS查询客户端
type WhoisClient struct {
	timeout time.Duration
}

// NewWhoisClient 创建新的WHOIS客户端
func NewWhoisClient(timeout time.Duration) *WhoisClient {
	return &WhoisClient{
		timeout: timeout,
	}
}

// QueryWhois 执行WHOIS查询，单次查询不重试
func (w *WhoisClient) QueryWhois(domain, server string, port int) (string, error) {
	address := net.JoinHostPort(server, fmt.Sprintf("%d", port))

	conn, err := net.DialTimeout("tcp", address, w.timeout)
	if err != nil {
		return "", fmt.Errorf("连接WHOIS服务器失败: %v", err)
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(w.timeout))

	query := domain + "\r\n"
	_, err = conn.Write([]byte(query))
	if err != nil {
		return "", fmt.Errorf("发送查询请求失败: %v", err)
	}

	response := make([]byte, 0, 4096)
	buffer := make([]byte, 1024)

	for {
		n, err := conn.Read(buffer)
		if err != nil {
			if n == 0 {
				break // 连接关闭
			}
			// 如果读取到部分数据，继续处理
			if len(response) > 0 {
				break
			}
			return "", fmt.Errorf("读取响应失败: %v", err)
		}
		response = append(response, buffer[:n]...)

		if len(response) > 100*1024 { // 100KB限制
			break
		}
	}

	if len(response) == 0 {
		return "", fmt.Errorf("WHOIS查询返回空响应")
	}

	return string(response), nil
}

// ParseWhoisResponse 解析WHOIS响应
func (w *WhoisClient) ParseWhoisResponse(domain, response string) *DomainInfo {
	info := &DomainInfo{
		Name:        domain,
		LastChecked: time.Now(),
		QueryMethod: "whois",
	}

	// 转换为小写便于匹配
	lowerResponse := strings.ToLower(response)

	// 检查域名状态
	info.Status = w.parseStatus(lowerResponse)

	// 解析注册商
	info.Registrar = w.parseRegistrar(response)

	// 检测TLD类型并设置不支持的数据提示
	tld := w.extractTLD(domain)

	// 解析日期
	info.CreatedDate = w.parseDate(response, []string{"creation date", "created", "registered"})
	info.ExpiryDate = w.parseDate(response, []string{"expiry date", "expires", "expiration date", "registry expiry date"})
	info.UpdatedDate = w.parseDate(response, []string{"updated date", "last updated", "modified"})

	// 为某些TLD设置不支持数据的提示
	w.setUnsupportedDataMessages(info, tld, response)

	// 解析名称服务器
	info.NameServers = w.parseNameServers(response)

	// 最终状态校验：只有在状态未知且有注册信息时，才设置为已注册
	// 不要覆盖已经正确检测到的特殊状态（如宽限期、赎回期、待删除等）
	if info.Status == StatusUnknown {
		hasValidRegistrar := info.Registrar != "" && !strings.Contains(info.Registrar, "不支持")
		hasNameServers := len(info.NameServers) > 0
		hasExpiryDate := info.ExpiryDate != nil
		hasCreatedDate := info.CreatedDate != nil

		// 如果有任何实际的注册信息，则认定为已注册
		if hasValidRegistrar || hasNameServers || hasExpiryDate || hasCreatedDate {
			info.Status = StatusRegistered
		} else {
			// 没有任何注册信息且状态未知
			logger.Warn("WHOIS无法解析域名 %s 状态，响应长度: %d", domain, len(response))
		}
	}

	return info
}

// parseStatus 解析域名状态
func (w *WhoisClient) parseStatus(response string) DomainStatus {
	// 首先检查特殊错误情况
	lowerResponse := strings.ToLower(response)

	// 查询频率限制
	if strings.Contains(lowerResponse, "number of allowed queries exceeded") ||
		strings.Contains(lowerResponse, "query limit") ||
		strings.Contains(lowerResponse, "rate limit") ||
		strings.Contains(lowerResponse, "too many requests") {
		return StatusError
	}

	// IP黑名单
	if strings.Contains(lowerResponse, "blacklisted") ||
		strings.Contains(lowerResponse, "blocked") ||
		strings.Contains(lowerResponse, "access denied") {
		return StatusError
	}

	// 服务不可用
	if strings.Contains(lowerResponse, "service unavailable") ||
		strings.Contains(lowerResponse, "temporarily unavailable") ||
		strings.Contains(lowerResponse, "server error") {
		return StatusError
	}

	patterns := config.GetDetectionPatterns()

	// 检查是否可注册（优先级最高）
	for _, pattern := range patterns.AvailablePatterns {
		if strings.Contains(response, strings.ToLower(pattern)) {
			return StatusAvailable
		}
	}

	// 检查宽限期模式（从配置文件获取）
	for _, pattern := range patterns.GracePatterns {
		if strings.Contains(response, strings.ToLower(pattern)) {
			return StatusGrace
		}
	}

	// 检查赎回期
	for _, pattern := range patterns.RedemptionPatterns {
		if strings.Contains(response, strings.ToLower(pattern)) {
			return StatusRedemption
		}
	}

	// 检查待删除状态
	for _, pattern := range patterns.PendingDeletePatterns {
		if strings.Contains(response, strings.ToLower(pattern)) {
			return StatusPendingDelete
		}
	}

	// 过期信息统一视为宽限期
	for _, pattern := range patterns.ExpiredPatterns {
		if strings.Contains(response, strings.ToLower(pattern)) {
			if w.isExpired(response) {
				return StatusGrace
			}
		}
	}

	// 如果包含注册信息，则认为已注册
	for _, pattern := range patterns.RegisteredPatterns {
		if strings.Contains(response, strings.ToLower(pattern)) {
			return StatusRegistered
		}
	}

	return StatusUnknown
}

// parseRegistrar 解析注册商
func (w *WhoisClient) parseRegistrar(response string) string {
	patterns := []string{
		`(?i)registrar:\s*(.+)`,
		`(?i)registrar organization:\s*(.+)`,
		`(?i)sponsoring registrar:\s*(.+)`,
		`(?i)sponsoring Registrar:\s*(.+)`,
		`(?i)Registrar Name:\s*(.+)`,
		`(?i)Organization:\s*(.+)`,
		// .ru 格式
		`(?i)registrar:\s*(.+)`,
		// .jp 格式 (日文和英文)
		`(?i)\[Name\]\s*(.+)`,
		`(?i)\[登録者名\]\s*(.+)`,
		// .kr 格式
		`(?i)등록대행자\s*:\s*(.+)`,
		`(?i)Authorized Agency\s*:\s*(.+)`,
		// .hk 格式
		`(?i)Registrar Name:\s*(.+)`,
		// houzhui.txt 特殊格式
		// .ax 格式
		`(?i)registrar\.+:\s*(.+)`,
		// .fi 格式 (带多个点)
		`(?i)registrar\.+:\s*(.+)`,
		// .bn 格式 (单独一行)
		`(?i)^Registrar:\s*(.+)`,
		// .sn、.ga 格式（法语）
		`(?i)Registrar:\s*(.+)`,
		// .tr 格式
		`(?i)Organization Name:\s*(.+)`,
		// .gg 格式
		`(?i)Registrar:\s*(.+)`,
		// .ro 格式
		`(?i)Registrar:\s*(.+)`,
		// .kz 格式
		`(?i)Current Registar:\s*(.+)`,
		`(?i)Registar created:\s*(.+)`,
		// .rs 格式
		`(?i)^Registrar:\s*(.+)`,
		// .tg 格式
		`(?i)Registrar:\.+(.+)`,
		// .lu 格式
		`(?i)registrar-name:\s*(.+)`,
		// .lv 格式 (section based)
		`(?i)\[Registrar\][\s\S]*?Name:\s*(.+)`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(response)
		if len(matches) > 1 {
			registrar := strings.TrimSpace(matches[1])
			// 清理括号内容
			if idx := strings.Index(registrar, "("); idx != -1 {
				registrar = strings.TrimSpace(registrar[:idx])
			}
			return registrar
		}
	}

	return ""
}

// parseDate 解析日期
func (w *WhoisClient) parseDate(response string, keywords []string) *time.Time {
	// 通用日期模式
	for _, keyword := range keywords {
		pattern := fmt.Sprintf(`(?i)%s:\s*([^\r\n]+)`, regexp.QuoteMeta(keyword))
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(response)

		if len(matches) > 1 {
			dateStr := strings.TrimSpace(matches[1])
			if date := w.parseDateTime(dateStr); date != nil {
				return date
			}
		}
	}

	// 特定TLD的日期模式
	specialPatterns := w.getSpecialDatePatterns(keywords)
	for _, pattern := range specialPatterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(response)
		if len(matches) > 1 {
			dateStr := strings.TrimSpace(matches[1])
			if date := w.parseDateTime(dateStr); date != nil {
				return date
			}
		}
	}

	return nil
}

// getSpecialDatePatterns 获取特定TLD的日期模式
func (w *WhoisClient) getSpecialDatePatterns(keywords []string) []string {
	patterns := []string{}

	// 根据关键词类型返回不同的模式
	for _, keyword := range keywords {
		switch strings.ToLower(keyword) {
		case "creation date", "created", "registered":
			patterns = append(patterns, []string{
				// .ru 格式
				`(?i)created:\s*([^\r\n]+)`,
				// .cn 格式
				`(?i)Registration Time:\s*([^\r\n]+)`,
				// .hk 格式
				`(?i)Domain Name Commencement Date:\s*([^\r\n]+)`,
				// .jp 格式
				`(?i)\[登録年月日\]\s*([^\r\n]+)`,
				// .kr 格式
				`(?i)등록일\s*:\s*([^\r\n]+)`,
				`(?i)Registered Date\s*:\s*([^\r\n]+)`,
				// .de 格式
				`(?i)Changed:\s*([^\r\n]+)`,
				// .au 格式
				`(?i)Last Modified:\s*([^\r\n]+)`,

				// houzhui.txt 特殊格式 - 创建日期
				// .ax 格式
				`(?i)created\.+:\s*([^\r\n]+)`,
				// .bn 格式 (带tab和空格)
				`(?i)Creation Date:\s+([^\r\n]+)`,
				// .cl 格式
				`(?i)Creation date:\s*([^\r\n]+)`,
				// .cr, .ee, .ve, .ls, .mk, .mo, .mw 格式
				`(?i)registered:\s*([^\r\n]+)`,
				// .fi 格式 (带多个点)
				`(?i)created\.+:\s*([^\r\n]+)`,
				// .is 格式
				`(?i)created:\s*([^\r\n]+)`,
				// .it 格式
				`(?i)Created:\s*([^\r\n]+)`,
				// .kg 格式
				`(?i)Record created:\s*([^\r\n]+)`,
				// .kz 格式
				`(?i)Domain created:\s*([^\r\n]+)`,
				// .pl 格式
				`(?i)created:\s*([^\r\n]+)`,
				// .pp.ua, .biz.ua 格式
				`(?i)Created On:\s*([^\r\n]+)`,
				// .rs 格式
				`(?i)Registration date:\s*([^\r\n]+)`,
				// .pt 格式（葡萄牙语）
				`(?i)Data de Registo:\s*([^\r\n]+)`,
				// .ro 格式
				`(?i)Registered On:\s*([^\r\n]+)`,
				// .sb 格式
				`(?i)Creation Date:\s*([^\r\n]+)`,
				// .sk 格式
				`(?i)Created:\s*([^\r\n]+)`,
				// .sn, .ga 格式（法语）
				`(?i)Date de création:\s*([^\r\n]+)`,
				// .tg 格式
				`(?i)Activation:\.+([^\r\n]+)`,
				// .ug 格式
				`(?i)Registered On:\s*([^\r\n]+)`,
				// .tr 格式
				`(?i)Created on\.+:\s*([^\r\n]+)`,
				// .gg 格式
				`(?i)Registered on\s+([^\r\n]+)`,
				// .br 格式
				`(?i)created:\s*([^\r\n]+)`,
				// .il 格式
				`(?i)assigned:\s*([^\r\n]+)`,
				// .sm 格式
				`(?i)Registration date:\s*([^\r\n]+)`,
				// .tn 格式
				`(?i)Creation date\.+:\s*([^\r\n]+)`,
				// .mo 格式
				`(?i)Record created on\s+([^\r\n]+)`,
			}...)
		case "expiry date", "expires", "expiration date", "registry expiry date":
			patterns = append(patterns, []string{
				// .ru 格式
				`(?i)paid-till:\s*([^\r\n]+)`,
				`(?i)free-date:\s*([^\r\n]+)`,
				// .cn 格式
				`(?i)Expiration Time:\s*([^\r\n]+)`,
				// .hk 格式
				`(?i)Expiry Date:\s*([^\r\n]+)`,
				// .jp 格式
				`(?i)\[有効期限\]\s*([^\r\n]+)`,
				// .kr 格式
				`(?i)사용 종료일\s*:\s*([^\r\n]+)`,
				`(?i)Expiration Date\s*:\s*([^\r\n]+)`,

				// houzhui.txt 特殊格式 - 过期日期
				// .ax 格式
				`(?i)expires\.+:\s*([^\r\n]+)`,
				// .bn 格式 (带tab和空格)
				`(?i)Expiration Date:\s+([^\r\n]+)`,
				// .cl 格式
				`(?i)Expiration date:\s*([^\r\n]+)`,
				// .cr, .ee, .ve, .ls, .mk, .mo, .mw 格式
				`(?i)expire:\s*([^\r\n]+)`,
				// .fi 格式 (带多个点)
				`(?i)expires\.+:\s*([^\r\n]+)`,
				// .is 格式
				`(?i)expires:\s*([^\r\n]+)`,
				// .it 格式
				`(?i)Expire Date:\s*([^\r\n]+)`,
				// .kg 格式
				`(?i)Record expires on:\s*([^\r\n]+)`,
				// .pl 格式
				`(?i)renewal date:\s*([^\r\n]+)`,
				// .pp.ua, .biz.ua 格式
				`(?i)Expiration Date:\s*([^\r\n]+)`,
				// .rs 格式
				`(?i)Expiration date:\s*([^\r\n]+)`,
				// .pt 格式（葡萄牙语）
				`(?i)Data de Expiração:\s*([^\r\n]+)`,
				// .ro 格式
				`(?i)Expires On:\s*([^\r\n]+)`,
				// .sb 格式
				`(?i)Registry Expiry Date:\s*([^\r\n]+)`,
				// .sk 格式
				`(?i)Valid Until:\s*([^\r\n]+)`,
				// .sn, .ga 格式（法语）
				`(?i)Date d'expiration:\s*([^\r\n]+)`,
				// .tg 格式
				`(?i)Expiration:\.+([^\r\n]+)`,
				// .ug 格式
				`(?i)Expires On:\s*([^\r\n]+)`,
				// .tm 格式
				`(?i)Expiry:\s*([^\r\n]+)`,
				// .tr 格式
				`(?i)Expires on\.+:\s*([^\r\n]+)`,
				// .ac.uk 格式
				`(?i)Renewal date:\s*([^\r\n]+)`,
				// .il 格式
				`(?i)validity:\s*([^\r\n]+)`,
				// .mo 格式
				`(?i)Record expires on\s+([^\r\n]+)`,
			}...)
		case "updated date", "last updated", "modified":
			patterns = append(patterns, []string{
				// .jp 格式
				`(?i)\[最終更新\]\s*([^\r\n]+)`,
				// .kr 格式
				`(?i)최근 정보 변경일\s*:\s*([^\r\n]+)`,
				`(?i)Last Updated Date\s*:\s*([^\r\n]+)`,
				// .de 格式
				`(?i)Changed:\s*([^\r\n]+)`,
				// .au 格式
				`(?i)Last Modified:\s*([^\r\n]+)`,

				// houzhui.txt 特殊格式 - 更新日期
				// .ax 格式
				`(?i)modified\.+:\s*([^\r\n]+)`,
				// .bn 格式 (带tab和空格)
				`(?i)Modified Date:\s+([^\r\n]+)`,
				// .cr, .ve 格式
				`(?i)changed:\s*([^\r\n]+)`,
				// .ee 格式
				`(?i)changed:\s*([^\r\n]+)`,
				// .fi 格式 (带多个点)
				`(?i)modified\.+:\s*([^\r\n]+)`,
				// .it 格式
				`(?i)Last Update:\s*([^\r\n]+)`,
				// .kz 格式 (带空格)
				`(?i)Last modified\s*:\s*([^\r\n]+)`,
				// .pp.ua, .biz.ua 格式
				`(?i)Last Updated On:\s*([^\r\n]+)`,
				// .rs 格式
				`(?i)Modification date:\s*([^\r\n]+)`,
				// .sk 格式
				`(?i)Updated:\s*([^\r\n]+)`,
				// .sn, .ga 格式（法语）
				`(?i)Dernière modification:\s*([^\r\n]+)`,
				// .br 格式
				`(?i)changed:\s*([^\r\n]+)`,
				// .qa 格式
				`(?i)Last Modified:\s*([^\r\n]+)`,
				// .lv 格式
				`(?i)Updated:\s*([^\r\n]+)`,
			}...)
		}
	}

	return patterns
}

// parseDateTime 解析日期时间字符串
func (w *WhoisClient) parseDateTime(dateStr string) *time.Time {
	// 清理日期字符串
	dateStr = strings.TrimSpace(dateStr)
	dateStr = regexp.MustCompile(`\s+`).ReplaceAllString(dateStr, " ")

	// 移除时区信息如 (JST)
	dateStr = regexp.MustCompile(`\s*\([^)]+\)\s*$`).ReplaceAllString(dateStr, "")

	formats := []string{
		// 标准ISO格式
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05.000Z",
		"2006-01-02T15:04:05+07:00",
		"2006-01-02T15:04:05-07:00",
		"2006-01-02 15:04:05",
		"2006-01-02",

		// 中文格式 (.cn)
		"2006-01-02 15:04:05",
		"2006-01-02",

		// 日文格式 (.jp)
		"2006/01/02",
		"2006/01/02 15:04:05",

		// 韩文格式 (.kr)
		"2006. 01. 02.",
		"2006. 01. 02",

		// 英文格式 (.hk)
		"02-01-2006",
		"2-1-2006",
		"02-1-2006",
		"2-01-2006",

		// 其他常见格式
		"02-Jan-2006",
		"January 02 2006",
		"Jan 02 2006",
		"02/01/2006",
		"01/02/2006",
		"2006.01.02",
		"2006.1.2",

		// houzhui.txt 特殊格式
		// .ax 格式 (dd.mm.yyyy)
		"02.01.2006",
		// .cl, .rs 格式 (yyyy-mm-dd HH:MM:SS TIMEZONE)
		"2006-01-02 15:04:05 MST",
		"2006-01-02 15:04:05 -0700",
		// .cr 格式 (dd.mm.yyyy)
		"02.01.2006",
		// .ee 格式 (yyyy-mm-dd HH:MM:SS +TZ)
		"2006-01-02 15:04:05 -07:00",
		"2006-01-02 15:04:05 +07:00",
		// .is 格式 (Month dd yyyy)
		"January 2 2006",
		"Jan 2 2006",
		// .kg 格式 (Day Month dd HH:MM:SS yyyy)
		"Mon Jan 2 15:04:05 2006",
		// .kz 格式 (yyyy-mm-dd HH:MM:SS (TZ))
		"2006-01-02 15:04:05 (MST+0:00)",
		"2006-01-02 15:04:05 (GMT+0:00)",
		// .pl 格式 (yyyy.mm.dd HH:MM:SS)
		"2006.01.02 15:04:05",
		// .pp.ua 格式 (dd-Mon-yyyy HH:MM:SS UTC)
		"02-Jan-2006 15:04:05 UTC",
		// .pt 格式 (yyyy-mm-dd)
		"2006-01-02",
		// .sb 格式 (yyyy-mm-ddTHH:MM:SSZ)
		"2006-01-02T15:04:05Z",
		// .sk 格式 (yyyy-mm-dd)
		"2006-01-02",
		// .sn, .ga 格式 (yyyy-mm-ddTHH:MM:SS.ffffffZ)
		"2006-01-02T15:04:05.999999Z",
		"2006-01-02T15:04:05.453646Z",
		// .tg 格式 (yyyy-mm-dd)
		"2006-01-02",
		// .ug 格式 (yyyy-mm-dd)
		"2006-01-02",
		// .tm 格式 (yyyy-mm-dd)
		"2006-01-02",
		// .tr 格式 (yyyy-Mon-dd)
		"2006-Jan-02",
		"2006-Jan-02.",
		// .ac.uk 格式 (Day ddth Mon yyyy)
		"Monday 2nd Jan 2006",
		// .gg 格式 (ddth Month yyyy at HH:MM:SS.000)
		"2nd January 2006",
		"2nd January 2006 at 15:04:05.000",
		// .il 格式 (dd-mm-yyyy)
		"02-01-2006",
		// .sm 格式 (dd/mm/yyyy)
		"02/01/2006",
		// .tn 格式 (dd-mm-yyyy HH:MM:SS TIMEZONE)
		"02-01-2006 15:04:05 GMT+1",
		"04-06-2024 06:53:01 GMT+1",
		// .biz.ua 格式 (dd-Mon-yyyy HH:MM:SS UTC)
		"02-Jan-2006 15:04:05 UTC",
		// .bn 格式 (dd-Mon-yyyy HH:MM:SS)
		"18-May-2020 00:00:00",
		"02-Jan-2006 00:00:00",
		"02-Jan-2006 15:04:05",
		// .fi 格式 (d.m.yyyy HH:MM:SS)
		"3.5.2016 15:48:12",
		"2.1.2006 15:04:05",
		// .rs 格式 (dd.mm.yyyy HH:MM:SS)
		"03.09.2010 12:25:53",
		"02.01.2006 15:04:05",
		// .mo 格式 (yyyy-mm-dd HH:MM:SS.ffffff)
		"2023-02-13 18:30:26.453646",
		"2006-01-02 15:04:05.999999",
		// .lv 格式 (yyyy-mm-ddTHH:MM:SS.ffffffZ)
		"2025-12-15T12:12:32.295699+00:00",
		"2006-01-02T15:04:05.999999+00:00",
	}

	for _, format := range formats {
		if date, err := time.Parse(format, dateStr); err == nil {
			return &date
		}
	}

	return nil
}

// parseNameServers 解析名称服务器
func (w *WhoisClient) parseNameServers(response string) []string {
	patterns := []string{
		`(?i)name server:\s*([^\r\n]+)`,
		`(?i)nameserver:\s*([^\r\n]+)`,
		`(?i)nserver:\s*([^\r\n]+)`,
		`(?i)dns:\s*([^\r\n]+)`,
		// .bn 格式 (Name Servers: 后多行)
		`(?i)Name Servers:\s*\n\s+([^\r\n]+)`,
		// .fi 格式 (nserver....:)
		`(?i)nserver\.+:\s*([^\r\n]+)`,
		// .kz 格式 (Primary/Secondary server)
		`(?i)(?:Primary|Secondary) server\.+:\s*([^\r\n]+)`,
		// .mo 格式 (Domain name servers: 后多行)
		`(?i)Domain name servers:[\s\S]*?([a-z0-9\-\.]+\.(?:com|net|org|cloudflare\.com|ns\.cloudflare\.com))`,
		// .rs 格式 (DNS:)
		`(?i)^DNS:\s*([^\r\n]+)`,
		// .tg 格式 (Name Server (DB):)
		`(?i)Name Server \(DB\):\.+([^\r\n]+)`,
		// .gg 格式 (Name servers: 后多行)
		`(?i)Name servers:\s*\n\s+([^\r\n]+)`,
		// .lu 格式 (nserver:)
		`(?i)^nserver:\s*([^\r\n]+)`,
		// .lv 格式 ([Nservers] section)
		`(?i)\[Nservers\][\s\S]*?Nserver:\s*([^\r\n]+)`,
	}

	var nameServers []string
	seen := make(map[string]bool)

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindAllStringSubmatch(response, -1)

		for _, match := range matches {
			if len(match) > 1 {
				ns := strings.TrimSpace(strings.ToLower(match[1]))
				// 清理可能的IP地址和额外信息
				ns = strings.Split(ns, " ")[0]
				ns = strings.Split(ns, "\t")[0]
				// 移除 [OK] 等标记
				ns = strings.TrimSuffix(ns, "[ok]")
				ns = strings.TrimSpace(ns)
				if ns != "" && !seen[ns] && strings.Contains(ns, ".") {
					nameServers = append(nameServers, ns)
					seen[ns] = true
				}
			}
		}
	}

	return nameServers
}

// isExpired 检查域名是否已过期
func (w *WhoisClient) isExpired(response string) bool {
	expiryPatterns := []string{
		`(?i)expiry date:\s*([^\r\n]+)`,
		`(?i)expires:\s*([^\r\n]+)`,
		`(?i)expiration date:\s*([^\r\n]+)`,
		`(?i)registry expiry date:\s*([^\r\n]+)`,
	}

	for _, pattern := range expiryPatterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(response)

		if len(matches) > 1 {
			dateStr := strings.TrimSpace(matches[1])
			if date := w.parseDateTime(dateStr); date != nil {
				return date.Before(time.Now())
			}
		}
	}

	return false
}

// extractTLD 从域名中提取TLD
func (w *WhoisClient) extractTLD(domain string) string {
	parts := strings.Split(domain, ".")
	if len(parts) >= 2 {
		return strings.ToLower(parts[len(parts)-1])
	}
	return ""
}

// setUnsupportedDataMessages 为不支持的数据设置提示信息
func (w *WhoisClient) setUnsupportedDataMessages(info *DomainInfo, tld, response string) {
	// 只有在域名状态不是available或error时才设置不支持信息
	// 如果域名可注册或查询失败，就不需要显示这些信息
	if info.Status == StatusAvailable || info.Status == StatusError {
		return
	}

	// 检查注册商信息
	if info.Registrar == "" {
		switch tld {
		case "de":
			info.Registrar = "该后缀不支持注册商信息"
		case "jp":
			if !strings.Contains(response, "[Name]") && !strings.Contains(response, "GMO") {
				info.Registrar = "该后缀不支持注册商信息"
			}
		default:
			// 通用检查：如果没有找到常见的注册商字段
			if !w.hasRegistrarInfo(response, tld) {
				info.Registrar = "该后缀不支持注册商信息"
			}
		}
	}

	// 检查创建时间
	if info.CreatedDate == nil {
		switch tld {
		case "de":
			info.CreatedDate = nil // .de 不支持创建时间
		case "ru":
			if !strings.Contains(response, "created:") && !strings.Contains(response, "registered:") {
				// 设置一个特殊标记，前端会显示为不支持
				info.CreatedDate = nil
			}
		}
	}

	// 检查过期时间
	if info.ExpiryDate == nil {
		switch tld {
		case "de":
			info.ExpiryDate = nil // .de 不支持过期时间
		case "jp":
			if !strings.Contains(response, "使用終了日") && !strings.Contains(response, "expires") {
				info.ExpiryDate = nil
			}
		}
	}

	// 检查更新时间
	if info.UpdatedDate == nil {
		// 大部分TLD都支持更新时间，只有少数不支持
		switch tld {
		case "some_unsupported_tld": // 示例，实际根据需要添加
			info.UpdatedDate = nil
		}
	}
}

// hasRegistrarInfo 检查响应中是否包含注册商信息
func (w *WhoisClient) hasRegistrarInfo(response, tld string) bool {
	// 根据不同TLD检查是否应该有注册商信息
	switch tld {
	case "de":
		// .de 域名不提供注册商信息
		return false
	case "cn", "com.cn", "net.cn", "org.cn":
		// .cn 域名应该有 "Sponsoring Registrar" 信息
		return strings.Contains(response, "Sponsoring Registrar") ||
			strings.Contains(response, "sponsoring registrar")
	case "jp":
		// .jp 域名通常不直接显示注册商，而是显示代理信息
		return strings.Contains(response, "[Name]") ||
			strings.Contains(response, "GMO")
	case "kr":
		// .kr 域名有注册代行者信息
		return strings.Contains(response, "등록대행자") ||
			strings.Contains(response, "Authorized Agency")
	case "hk":
		// .hk 域名有注册商信息
		return strings.Contains(response, "Registrar Name")
	case "ru":
		// .ru 域名有注册商信息
		return strings.Contains(response, "registrar:")
	case "au":
		// .au 域名有注册商信息
		return strings.Contains(response, "Registrar Name")
	default:
		// 其他TLD默认应该有注册商信息
		return true
	}
}
