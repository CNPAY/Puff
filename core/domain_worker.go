package core

import (
	"context"
	"strings"
	"sync"
	"time"

	"Puff/config"
	"Puff/logger"
	"Puff/storage"
)

// DomainWorker 域名查询工作线程
// 每个域名有一个独立的worker，负责定期查询和保存结果
type DomainWorker struct {
	domain        string
	checker       *DomainChecker
	config        *config.Config
	semaphore     chan struct{} // 并发控制信号量
	stopCh        chan struct{}
	ctx           context.Context
	cancel        context.CancelFunc
	lastStatus    DomainStatus
	statusChange  chan<- StatusChangeEvent // 状态变化通知通道
	notify        bool                     // 是否启用通知
	queryRecorder func(string)             // 查询记录函数
	isFirstQuery  bool                     // 是否为首次查询
}

// NewDomainWorker 创建新的域名工作线程
func NewDomainWorker(
	domain string,
	checker *DomainChecker,
	cfg *config.Config,
	semaphore chan struct{},
	statusChange chan<- StatusChangeEvent,
	notify bool,
	queryRecorder func(string),
) *DomainWorker {
	ctx, cancel := context.WithCancel(context.Background())

	// 检查是否为首次查询
	isFirstQuery := true
	result, err := storage.GetDomainResult(strings.ToLower(strings.TrimSpace(domain)))
	if err == nil && result != nil {
		isFirstQuery = false
	}

	return &DomainWorker{
		domain:        strings.ToLower(strings.TrimSpace(domain)),
		checker:       checker,
		config:        cfg,
		semaphore:     semaphore,
		stopCh:        make(chan struct{}),
		ctx:           ctx,
		cancel:        cancel,
		lastStatus:    StatusUnknown,
		statusChange:  statusChange,
		notify:        notify,
		queryRecorder: queryRecorder,
		isFirstQuery:  isFirstQuery,
	}
}

// Start 启动worker
func (w *DomainWorker) Start() {
	go w.run()
}

// Stop 停止worker（立即取消正在进行的查询）
func (w *DomainWorker) Stop() {
	logger.Debug("停止域名 %s 的worker", w.domain)
	w.cancel() // 取消上下文，中断正在进行的查询
	select {
	case <-w.stopCh:
		// 已经关闭
	default:
		close(w.stopCh)
	}
}

// run worker主循环
func (w *DomainWorker) run() {
	logger.Debug("域名Worker启动: %s", w.domain)

	// 立即执行第一次查询
	w.executeQuery()

	// 进入定期查询循环
	for {
		// 计算下次查询时间
		nextCheck := w.calculateNextCheckTime()
		waitDuration := time.Until(nextCheck)

		if waitDuration < 0 {
			waitDuration = 0
		}

		logger.Debug("域名 %s 下次查询时间: %s (等待 %v)", w.domain, nextCheck.Format("2006-01-02 15:04:05"), waitDuration)

		// 如果等待时间大于10秒，使用ticker定期检查配置变化
		if waitDuration > 10*time.Second {
			ticker := time.NewTicker(10 * time.Second)
			deadline := time.After(waitDuration)

		checkLoop:
			for {
				select {
				case <-deadline:
					// 到达预定查询时间
					ticker.Stop()
					w.executeQuery()
					break checkLoop
				case <-ticker.C:
					// 定期检查是否需要提前查询（配置变更）
					newNextCheck := w.calculateNextCheckTime()

					// 如果新的检查时间比当前时间早，或者比之前的计划时间早很多，说明配置变了
					// 注意：这里简单判断如果新时间已过，立即执行
					if newNextCheck.Before(time.Now()) {
						logger.Info("域名 %s 检测到配置变更(时间已过)，提前开始查询", w.domain)
						ticker.Stop()
						w.executeQuery()
						break checkLoop
					}

					// 计算当前还需等待的时间
					newWaitDuration := time.Until(newNextCheck)

					// 如果 newNextCheck 明显早于 nextCheck（比如早了超过 5 秒），说明间隔缩短了
					if newNextCheck.Before(nextCheck.Add(-5 * time.Second)) {
						logger.Info("域名 %s 检测到间隔缩短，重置等待时间: %v -> %v", w.domain, nextCheck.Sub(time.Now()), newWaitDuration)
						ticker.Stop()

						// 如果时间已经过了（或非常接近），直接执行
						if newWaitDuration <= 0 {
							w.executeQuery()
						} else {
							// 否则，直接跳出内部循环，外层循环会重新计算并设置新的计时器
						}
						break checkLoop
					}
				case <-w.stopCh:
					ticker.Stop()
					logger.Debug("域名Worker停止: %s", w.domain)
					return
				case <-w.ctx.Done():
					ticker.Stop()
					logger.Debug("域名Worker上下文取消: %s", w.domain)
					return
				}
			}
		} else {
			// 等待时间较短，直接等待
			select {
			case <-time.After(waitDuration):
				// 时间到，执行查询
				w.executeQuery()
			case <-w.stopCh:
				// 收到停止信号
				logger.Debug("域名Worker停止: %s", w.domain)
				return
			case <-w.ctx.Done():
				// 上下文取消
				logger.Debug("域名Worker上下文取消: %s", w.domain)
				return
			}
		}
	}
}

// executeQuery 执行查询并保存结果
func (w *DomainWorker) executeQuery() {
	// 记录准备开始查询的时间
	prepareTime := time.Now()

	// 记录查询开始（用于通知聚合）
	if w.queryRecorder != nil {
		w.queryRecorder(w.domain)
	}

	// 获取当前的信号量（并发控制）
	// 注意：必须先获取当前的信号量引用，避免在等待期间信号量被更新导致死锁
	// 如果在等待期间 WorkerManager 更新了信号量，w.semaphore 会指向新的 channel
	// 但我们必须确保 Acquire 和 Release 操作的是同一个 channel
	sem := w.semaphore

	// 阻塞等待获取信号量
	logger.Debug("域名 %s 准备获取信号量 (容量: %d)", w.domain, cap(sem))
	sem <- struct{}{}

	// 查询完成后释放信号量（使用同一个信号量引用）
	defer func() {
		<-sem
	}()

	startTime := time.Now()
	// 计算等待时间
	waitTime := startTime.Sub(prepareTime)
	if waitTime > 100*time.Millisecond {
		logger.Debug("域名 %s 等待并发槽位耗时: %v", w.domain, waitTime)
	}

	logger.Info("域名 %s 开始查询，时间: %s", w.domain, startTime.Format("2006-01-02 15:04:05"))

	// 从数据库读取之前的状态
	previousResult, err := storage.GetDomainResult(w.domain)
	var previousStatus DomainStatus = StatusUnknown
	if err == nil && previousResult != nil {
		previousStatus = DomainStatus(previousResult.Status)
	}

	// 执行查询（带重试）
	info := w.queryWithRetry()

	// 保存到数据库
	w.saveToDatabase(info)

	// 检查状态变化并发送通知
	// 只有当有明确的前一个状态，且状态发生变化时才通知
	if w.isFirstQuery {
		// 首次查询，不通知
		logger.Info("域名 %s 首次查询，状态: %s，不发送通知", w.domain, info.Status)
		w.isFirstQuery = false
	} else if previousStatus != StatusUnknown && previousStatus != info.Status {
		// 状态变化，发送通知
		w.notifyStatusChange(previousStatus, info.Status, info)
	}

	// 更新最后状态
	w.lastStatus = info.Status

	endTime := time.Now()
	duration := endTime.Sub(startTime)
	logger.Info("域名 %s 查询完成，状态: %s，开始: %s，结束: %s，耗时: %v",
		w.domain, info.Status,
		startTime.Format("2006-01-02 15:04:05"),
		endTime.Format("2006-01-02 15:04:05"),
		duration)
}

// queryWithRetry 带重试的查询
func (w *DomainWorker) queryWithRetry() *DomainInfo {
	const maxRetries = 3
	var failureReasons []string

	for attempt := 1; attempt <= maxRetries; attempt++ {
		// 检查是否需要停止
		select {
		case <-w.ctx.Done():
			return &DomainInfo{
				Name:         w.domain,
				Status:       StatusError,
				ErrorMessage: "查询被取消",
				LastChecked:  time.Now(),
			}
		default:
		}

		// 执行查询
		info := w.checker.CheckDomain(w.domain)

		// 查询成功
		if info.Status != StatusError {
			return info
		}

		// 记录失败原因（简化错误信息）
		if info.ErrorMessage != "" {
			// 简化错误信息：移除重复的"WHOIS连接失败: "前缀
			simplified := strings.TrimPrefix(info.ErrorMessage, "WHOIS连接失败: ")
			simplified = strings.TrimPrefix(simplified, "RDAP查询失败: ")
			failureReasons = append(failureReasons, simplified)
		}

		// 如果不是最后一次尝试，等待后重试
		if attempt < maxRetries {
			waitTime := time.Duration(attempt) * 500 * time.Millisecond
			logger.Debug("域名 %s 第%d次查询失败，%v后重试", w.domain, attempt, waitTime)

			select {
			case <-time.After(waitTime):
				// 继续重试
			case <-w.ctx.Done():
				// 被取消
				return info
			}
		}
	}

	// 所有重试都失败，格式化错误信息
	uniqueReasons := uniqueStrings(failureReasons)
	var errorMsg string
	if len(uniqueReasons) == 1 {
		errorMsg = "连续3次查询失败: " + uniqueReasons[0]
	} else if len(uniqueReasons) > 1 {
		errorMsg = "连续3次查询失败: " + strings.Join(uniqueReasons, "; ")
	} else {
		errorMsg = "连续3次查询失败，原因未知"
	}

	logger.Error("域名 %s %s", w.domain, errorMsg)

	return &DomainInfo{
		Name:         w.domain,
		Status:       StatusError,
		ErrorMessage: errorMsg,
		LastChecked:  time.Now(),
	}
}

// saveToDatabase 保存查询结果到数据库
func (w *DomainWorker) saveToDatabase(info *DomainInfo) {
	if info == nil {
		return
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

	// 处理注册商信息：错误状态不显示"不支持"提示
	registrar := info.Registrar
	if registrar == "" && info.Status != StatusAvailable && info.Status != StatusError && info.Status != StatusUnknown {
		registrar = "该后缀不支持注册商信息"
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
	} else {
		logger.Debug("域名 %s 结果已保存到数据库", info.Name)
	}
}

// calculateNextCheckTime 计算下次查询时间（支持热重载）
func (w *DomainWorker) calculateNextCheckTime() time.Time {
	// 从数据库读取最后检查时间和状态
	result, err := storage.GetDomainResult(w.domain)
	if err != nil {
		// 如果读取失败，立即查询
		logger.Debug("读取域名 %s 结果失败，立即查询: %v", w.domain, err)
		return time.Now()
	}

	if result == nil {
		// 没有历史记录，立即查询
		return time.Now()
	}

	// 根据状态计算间隔时间（使用最新配置）
	interval := w.getIntervalByStatus(DomainStatus(result.Status))

	// 计算基于最后检查时间的下次查询时间
	nextCheck := result.LastChecked.Add(interval)
	now := time.Now()

	// 如果下次查询时间已经过去，立即查询
	if nextCheck.Before(now) {
		logger.Debug("域名 %s 下次查询时间已过期，立即查询", w.domain)
		return now
	}

	// 如果距离上次查询的时间已经超过了新的间隔，立即查询（支持缩短间隔）
	timeSinceLastCheck := now.Sub(result.LastChecked)
	if timeSinceLastCheck >= interval {
		logger.Debug("域名 %s 距离上次查询已超过新间隔(%v)，立即查询", w.domain, interval)
		return now
	}

	return nextCheck
}

// getIntervalByStatus 根据状态获取查询间隔
func (w *DomainWorker) getIntervalByStatus(status DomainStatus) time.Duration {
	// 优先使用配置的全局查询间隔，解决热重载失效问题
	// 用户希望通过设置界面的"检查间隔"来控制所有域名的查询频率
	baseInterval := w.config.Monitor.CheckInterval

	switch status {
	case StatusAvailable:
		// 可注册：使用配置间隔
		return baseInterval

	case StatusPendingDelete:
		// 待删除：使用配置间隔
		return baseInterval

	case StatusRedemption:
		// 赎回期：使用配置间隔
		return baseInterval

	case StatusGrace:
		// 宽限期：使用配置间隔
		return baseInterval

	case StatusError:
		// 错误状态：1小时后重试 (保持较长间隔避免频繁报错)
		return 1 * time.Hour

	case StatusRegistered:
		// 已注册：使用配置的间隔
		return baseInterval

	default:
		// 其他状态：使用配置的间隔
		return baseInterval
	}
}

// notifyStatusChange 发送状态变化通知
func (w *DomainWorker) notifyStatusChange(oldStatus, newStatus DomainStatus, info *DomainInfo) {
	// 检查是否需要发送通知
	if !w.notify {
		return
	}

	// 如果是从查询失败状态恢复，不发送通知
	if oldStatus == StatusError {
		logger.Info("域名 %s 从失败状态恢复，不发送通知", w.domain)
		return
	}

	// 检查新状态是否需要通知
	statusInfo := GetStatusInfo(newStatus)
	if !statusInfo.ShouldNotify {
		return
	}

	event := StatusChangeEvent{
		Domain:     w.domain,
		OldStatus:  oldStatus,
		NewStatus:  newStatus,
		Timestamp:  time.Now(),
		Message:    GetStatusChangeMessage(w.domain, oldStatus, newStatus),
		DomainInfo: info,
	}

	// 非阻塞发送
	select {
	case w.statusChange <- event:
		logger.Info("域名 %s 状态变化通知已发送: %s -> %s", w.domain, oldStatus, newStatus)
	default:
		logger.Warn("通知队列已满，丢弃域名 %s 的状态变化通知", w.domain)
	}
}

// uniqueStrings 字符串去重
func uniqueStrings(strs []string) []string {
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

// WorkerManager worker管理器
type WorkerManager struct {
	workers       map[string]*DomainWorker
	mu            sync.RWMutex
	checker       *DomainChecker
	config        *config.Config
	semaphore     chan struct{} // 并发控制信号量
	statusCh      chan StatusChangeEvent
	queryRecorder func(string) // 查询记录函数
}

// NewWorkerManager 创建worker管理器
func NewWorkerManager(checker *DomainChecker, cfg *config.Config, statusCh chan StatusChangeEvent, queryRecorder func(string)) *WorkerManager {
	// 创建信号量，容量为并发限制
	semaphore := make(chan struct{}, cfg.Monitor.ConcurrentLimit)

	return &WorkerManager{
		workers:       make(map[string]*DomainWorker),
		checker:       checker,
		config:        cfg,
		semaphore:     semaphore,
		statusCh:      statusCh,
		queryRecorder: queryRecorder,
	}
}

// AddWorker 添加worker（避免重复添加）
func (m *WorkerManager) AddWorker(domain string, notify bool) {
	domain = strings.ToLower(strings.TrimSpace(domain))

	m.mu.Lock()
	defer m.mu.Unlock()

	// 如果已存在，不重复添加
	if _, exists := m.workers[domain]; exists {
		logger.Debug("域名 %s 的worker已存在，跳过添加", domain)
		return
	}

	// 创建新worker
	worker := NewDomainWorker(domain, m.checker, m.config, m.semaphore, m.statusCh, notify, m.queryRecorder)
	m.workers[domain] = worker

	// 启动worker
	worker.Start()
	logger.Info("域名 %s 的worker已启动", domain)
}

// RemoveWorker 移除worker
func (m *WorkerManager) RemoveWorker(domain string) {
	domain = strings.ToLower(strings.TrimSpace(domain))

	m.mu.Lock()
	defer m.mu.Unlock()

	if worker, exists := m.workers[domain]; exists {
		worker.Stop()
		delete(m.workers, domain)
		logger.Info("域名 %s 的worker已停止", domain)
	}
}

// StopAll 停止所有worker
func (m *WorkerManager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	logger.Info("正在停止所有worker，共 %d 个", len(m.workers))

	for domain, worker := range m.workers {
		worker.Stop()
		logger.Debug("域名 %s 的worker已停止", domain)
	}

	m.workers = make(map[string]*DomainWorker)
	logger.Info("所有worker已停止")
}

// GetWorkerCount 获取worker数量
func (m *WorkerManager) GetWorkerCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.workers)
}

// UpdateConcurrentLimit 更新并发限制
func (m *WorkerManager) UpdateConcurrentLimit(limit int) {
	if limit <= 0 {
		limit = 1
	}

	// 创建新的信号量
	newSemaphore := make(chan struct{}, limit)

	m.mu.Lock()
	m.semaphore = newSemaphore

	// 更新所有worker的信号量引用
	for _, worker := range m.workers {
		worker.semaphore = newSemaphore
	}
	m.mu.Unlock()

	logger.Info("并发限制已更新为: %d", limit)
}

// UpdateConfig 更新配置
func (m *WorkerManager) UpdateConfig(cfg *config.Config) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.config = cfg
	// 更新所有worker的配置引用
	for _, worker := range m.workers {
		worker.config = cfg
	}
}
