package core

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"Puff/config"
	"Puff/logger"
	"Puff/storage"
)

// Monitor 域名监控器
type Monitor struct {
	checker       *DomainChecker
	config        *config.Config
	isRunning     bool
	mu            sync.RWMutex
	notifications chan StatusChangeEvent
	startTime     time.Time      // 启动时间
	workerManager *WorkerManager // worker管理器
	queryRecorder func(string)   // 查询记录函数
}

// NewMonitor 创建新的监控器
func NewMonitor(cfg *config.Config, queryRecorder func(string)) *Monitor {
	notifications := make(chan StatusChangeEvent, 1000)
	checker := NewDomainChecker(cfg)
	workerManager := NewWorkerManager(checker, cfg, notifications, queryRecorder)

	return &Monitor{
		checker:       checker,
		config:        cfg,
		notifications: notifications,
		startTime:     time.Now(),
		workerManager: workerManager,
		isRunning:     false,
		queryRecorder: queryRecorder,
	}
}

// LoadDomains 加载域名列表并启动workers（仅在启动时调用）
func (m *Monitor) LoadDomains() error {
	// 加载所有启用的域名
	domainEntries, err := storage.ListDomains(true)
	if err != nil {
		return fmt.Errorf("加载域名列表失败: %v", err)
	}

	// 验证并创建worker
	validCount := 0
	for _, entry := range domainEntries {
		domain := entry.Name
		if err := m.checker.ValidateDomain(domain); err != nil {
			logger.Warn("跳过无效域名 %s: %v", domain, err)
			continue
		}

		// 为该域名创建worker
		m.workerManager.AddWorker(domain, entry.Notify)
		validCount++
	}

	logger.Info("加载了 %d 个有效域名", validCount)
	return nil
}

// Start 启动监控
func (m *Monitor) Start() error {
	m.mu.Lock()
	if m.isRunning {
		m.mu.Unlock()
		return fmt.Errorf("监控器已在运行")
	}

	m.isRunning = true
	m.mu.Unlock()

	// 仅在启动时加载域名并启动workers
	if err := m.LoadDomains(); err != nil {
		logger.Warn("加载域名失败: %v", err)
	}

	workerCount := m.workerManager.GetWorkerCount()
	logger.Info("域名监控已启动，并发限制: %d, worker数量: %d", m.config.Monitor.ConcurrentLimit, workerCount)

	return nil
}

// Stop 停止监控
func (m *Monitor) Stop() {
	m.mu.Lock()
	if !m.isRunning {
		m.mu.Unlock()
		return
	}

	m.isRunning = false
	m.mu.Unlock()

	// 停止所有workers
	m.workerManager.StopAll()

	logger.Info("域名监控已停止")
}

// UpdateConfig 更新配置（支持热重载）
func (m *Monitor) UpdateConfig(cfg *config.Config) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.config = cfg

	// 1. 更新Checker配置（超时时间等）
	if m.checker != nil {
		m.checker.UpdateConfig(cfg)
	}

	// 2. 更新WorkerManager配置（查询间隔等）
	if m.workerManager != nil {
		m.workerManager.UpdateConfig(cfg)

		// 3. 更新并发限制
		// 注意：这里单独调用UpdateConcurrentLimit，因为UpdateConfig只更新config引用
		m.workerManager.UpdateConcurrentLimit(cfg.Monitor.ConcurrentLimit)
	}

	logger.Info("Monitor配置已更新: 间隔=%v, 并发=%d, 超时=%v",
		cfg.Monitor.CheckInterval,
		cfg.Monitor.ConcurrentLimit,
		cfg.Monitor.Timeout)
}

// IsRunning 检查是否正在运行
func (m *Monitor) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.isRunning
}

// GetDomains 获取域名列表（从数据库）
func (m *Monitor) GetDomains() []string {
	domainEntries, err := storage.ListDomains(true)
	if err != nil {
		logger.Error("获取域名列表失败: %v", err)
		return []string{}
	}

	domains := make([]string, 0, len(domainEntries))
	for _, entry := range domainEntries {
		domains = append(domains, entry.Name)
	}
	return domains
}

// GetDomainInfo 获取域名信息（从数据库）
func (m *Monitor) GetDomainInfo(domain string) (*DomainInfo, error) {
	domain = strings.ToLower(strings.TrimSpace(domain))

	// 从数据库读取
	result, err := storage.GetDomainResult(domain)
	if err != nil {
		return nil, fmt.Errorf("读取域名信息失败: %v", err)
	}

	if result == nil {
		// 数据库中没有记录，返回未知状态
		return &DomainInfo{
			Name:        domain,
			Status:      StatusUnknown,
			LastChecked: time.Now(),
			QueryMethod: "pending",
		}, nil
	}

	// 转换为DomainInfo
	return &DomainInfo{
		Name:         result.Domain,
		Status:       DomainStatus(result.Status),
		Registrar:    result.Registrar,
		CreatedDate:  result.CreatedAt,
		ExpiryDate:   result.ExpiryAt,
		UpdatedDate:  result.UpdatedAt,
		NameServers:  result.NameServers,
		LastChecked:  result.LastChecked,
		QueryMethod:  result.QueryMethod,
		WhoisRaw:     result.WhoisRaw,
		ErrorMessage: result.ErrorMessage,
	}, nil
}

// GetAllDomainInfo 获取所有域名信息（从数据库）
func (m *Monitor) GetAllDomainInfo() []*DomainInfo {
	// 从数据库读取所有结果
	results, err := storage.LoadDomainResults()
	if err != nil {
		logger.Error("读取域名结果失败: %v", err)
		return []*DomainInfo{}
	}

	// 转换为DomainInfo列表
	infos := make([]*DomainInfo, 0, len(results))
	for _, res := range results {
		infos = append(infos, &DomainInfo{
			Name:         res.Domain,
			Status:       DomainStatus(res.Status),
			Registrar:    res.Registrar,
			LastChecked:  res.LastChecked,
			QueryMethod:  res.QueryMethod,
			CreatedDate:  res.CreatedAt,
			ExpiryDate:   res.ExpiryAt,
			UpdatedDate:  res.UpdatedAt,
			NameServers:  res.NameServers,
			WhoisRaw:     res.WhoisRaw,
			ErrorMessage: res.ErrorMessage,
		})
	}

	return infos
}

// ForceCheck 强制检查指定域名（手动触发立即查询）
func (m *Monitor) ForceCheck(domain string) (*DomainInfo, error) {
	domain = strings.ToLower(strings.TrimSpace(domain))
	startTime := time.Now()
	logger.Info("强制检查域名 %s，开始时间: %s", domain, startTime.Format("2006-01-02 15:04:05"))

	// 记录查询开始（用于通知聚合）
	if m.queryRecorder != nil {
		m.queryRecorder(domain)
	}

	// 获取之前的状态
	previousResult, err := storage.GetDomainResult(domain)
	var previousStatus DomainStatus = StatusUnknown
	if err == nil && previousResult != nil {
		previousStatus = DomainStatus(previousResult.Status)
	}

	// 使用checker直接查询（带重试）
	info := m.queryDomainWithRetry(domain)

	// 保存到数据库
	m.saveResultToDB(info)

	endTime := time.Now()
	duration := endTime.Sub(startTime)
	logger.Info("域名 %s 查询完成，状态: %s，开始: %s，结束: %s，耗时: %v",
		domain, info.Status,
		startTime.Format("2006-01-02 15:04:05"),
		endTime.Format("2006-01-02 15:04:05"),
		duration)

	// 检查状态变化并发送通知
	if previousStatus != StatusUnknown && previousStatus != info.Status && previousStatus != StatusError {
		logger.Info("检测到域名 %s 状态变化: %s -> %s", domain, previousStatus, info.Status)

		// 检查是否需要通知
		statusInfo := GetStatusInfo(info.Status)
		if statusInfo.ShouldNotify {
			// 发送状态变化事件
			event := StatusChangeEvent{
				Domain:     domain,
				OldStatus:  previousStatus,
				NewStatus:  info.Status,
				Timestamp:  time.Now(),
				Message:    GetStatusChangeMessage(domain, previousStatus, info.Status),
				DomainInfo: info,
			}

			// 发送到通知通道
			select {
			case m.notifications <- event:
				logger.Info("域名 %s 状态变化通知已发送", domain)
			default:
				logger.Warn("通知队列已满，丢弃域名 %s 的状态变化通知", domain)
			}
		}
	}

	return info, nil
}

// queryDomainWithRetry 查询域名（带重试）
func (m *Monitor) queryDomainWithRetry(domain string) *DomainInfo {
	const maxRetries = 3
	var failureReasons []string

	for attempt := 1; attempt <= maxRetries; attempt++ {
		info := m.checker.CheckDomain(domain)

		// 查询成功
		if info.Status != StatusError {
			return info
		}

		// 记录失败原因
		if info.ErrorMessage != "" {
			failureReasons = append(failureReasons, info.ErrorMessage)
		}

		// 如果不是最后一次尝试，等待后重试
		if attempt < maxRetries {
			waitTime := time.Duration(attempt) * 500 * time.Millisecond
			logger.Debug("域名 %s 第%d次查询失败，%v后重试", domain, attempt, waitTime)
			time.Sleep(waitTime)
		}
	}

	// 所有重试都失败，格式化错误信息
	uniqueReasons := deduplicateStrings(failureReasons)
	var errorMsg string
	if len(uniqueReasons) == 1 {
		errorMsg = "连续3次查询失败: " + uniqueReasons[0]
	} else if len(uniqueReasons) > 1 {
		errorMsg = "连续3次查询失败: " + strings.Join(uniqueReasons, "; ")
	} else {
		errorMsg = "连续3次查询失败，原因未知"
	}

	return &DomainInfo{
		Name:         domain,
		Status:       StatusError,
		ErrorMessage: errorMsg,
		LastChecked:  time.Now(),
	}
}

// deduplicateStrings 字符串去重（辅助函数）
func deduplicateStrings(strs []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0)

	for _, s := range strs {
		if s != "" && !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}

	return result
}

// AddDomain 添加域名并启动worker（不重新加载所有域名）
func (m *Monitor) AddDomain(domain string, notify bool) error {
	domain = strings.ToLower(strings.TrimSpace(domain))

	// 验证域名格式
	if err := m.checker.ValidateDomain(domain); err != nil {
		return fmt.Errorf("无效的域名: %v", err)
	}

	m.mu.RLock()
	isRunning := m.isRunning
	m.mu.RUnlock()

	// 如果监控已启动，立即为该域名创建worker
	if isRunning {
		m.workerManager.AddWorker(domain, notify)
		logger.Info("域名 %s 已添加并启动worker", domain)
	} else {
		logger.Debug("监控未启动，域名 %s 将在启动时加载", domain)
	}

	return nil
}

// RemoveDomain 删除域名并停止worker
func (m *Monitor) RemoveDomain(domain string) {
	domain = strings.ToLower(strings.TrimSpace(domain))

	// 停止该域名的worker
	m.workerManager.RemoveWorker(domain)

	logger.Info("域名 %s 的worker已停止", domain)
}

// GetNotifications 获取通知通道
func (m *Monitor) GetNotifications() <-chan StatusChangeEvent {
	return m.notifications
}

// saveResultToDB 将单条结果写入数据库
func (m *Monitor) saveResultToDB(info *DomainInfo) {
	if info == nil {
		return
	}

	// 处理注册商信息：错误状态不显示"不支持"提示
	registrar := info.Registrar
	if registrar == "" && info.Status != StatusAvailable && info.Status != StatusError && info.Status != StatusUnknown {
		registrar = "该后缀不支持注册商信息"
	}

	// 转为北京时区
	loc := time.FixedZone("CST", 8*3600)
	toCST := func(t *time.Time) *time.Time {
		if t == nil {
			return nil
		}
		tt := t.In(loc)
		return &tt
	}

	res := storage.DomainResult{
		Domain:       strings.ToLower(info.Name),
		Status:       string(info.Status),
		Registrar:    registrar,
		LastChecked:  info.LastChecked.In(loc),
		QueryMethod:  info.QueryMethod,
		CreatedAt:    toCST(info.CreatedDate),
		ExpiryAt:     toCST(info.ExpiryDate),
		UpdatedAt:    toCST(info.UpdatedDate),
		NameServers:  info.NameServers,
		WhoisRaw:     info.WhoisRaw,
		ErrorMessage: info.ErrorMessage,
	}

	if err := storage.SaveDomainResult(res); err != nil {
		logger.Error("保存域名结果失败 %s: %v", info.Name, err)
	}
}

// GetStats 获取监控统计信息
func (m *Monitor) GetStats() map[string]interface{} {
	m.mu.RLock()
	isRunning := m.isRunning
	m.mu.RUnlock()

	// 从数据库读取所有结果
	results, err := storage.LoadDomainResults()
	if err != nil {
		logger.Error("读取域名结果失败: %v", err)
		results = make(map[string]storage.DomainResult)
	}

	// 获取域名列表
	domains := m.GetDomains()
	domainCount := len(domains)

	// 获取worker数量
	workerCount := m.workerManager.GetWorkerCount()

	stats := map[string]interface{}{
		"domain_count": domainCount,
		"is_running":   isRunning,
		"worker_count": workerCount,
		"uptime":       time.Since(m.startTime).String(),
	}

	// 统计各状态的域名数量
	statusCounts := make(map[DomainStatus]int)
	for _, r := range results {
		statusCounts[DomainStatus(r.Status)]++
	}
	stats["status_counts"] = statusCounts

	return stats
}

// GetChecker 获取域名检查器
func (m *Monitor) GetChecker() *DomainChecker {
	return m.checker
}
