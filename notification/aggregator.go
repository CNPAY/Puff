package notification

import (
	"sync"
	"time"

	"Puff/logger"
	"Puff/storage"
)

// NotificationAggregator 通知聚合器
// 实现智能合并逻辑：
// 1. 10秒内的多个状态变化合并发送
// 2. 如果8秒内没有新的域名进入查询，直接发送
type NotificationAggregator struct {
	mgr                *NotificationManager
	pendingEvents      []NotificationEvent
	lastQueryTime      time.Time
	groupTimer         *time.Timer
	mu                 sync.Mutex
	eventCh            chan NotificationEvent
	stopCh             chan struct{}
	isRunning          bool
	lastDomainQueryMap map[string]time.Time // 记录每个域名最后的查询时间
}

// NewNotificationAggregator 创建通知聚合器
func NewNotificationAggregator(mgr *NotificationManager) *NotificationAggregator {
	return &NotificationAggregator{
		mgr:                mgr,
		pendingEvents:      make([]NotificationEvent, 0),
		eventCh:            make(chan NotificationEvent, 1000),
		stopCh:             make(chan struct{}),
		lastDomainQueryMap: make(map[string]time.Time),
	}
}

// Start 启动聚合器
func (a *NotificationAggregator) Start() {
	a.mu.Lock()
	if a.isRunning {
		a.mu.Unlock()
		return
	}
	a.isRunning = true
	a.mu.Unlock()

	go a.run()
	logger.Info("通知聚合器已启动")
}

// Stop 停止聚合器
func (a *NotificationAggregator) Stop() {
	a.mu.Lock()
	if !a.isRunning {
		a.mu.Unlock()
		return
	}
	a.isRunning = false
	a.mu.Unlock()

	close(a.stopCh)
	logger.Info("通知聚合器已停止")
}

// AddEvent 添加通知事件
func (a *NotificationAggregator) AddEvent(event NotificationEvent) {
	if !a.isRunning {
		return
	}

	select {
	case a.eventCh <- event:
		// 成功加入队列
	default:
		logger.Warn("通知聚合器队列已满，丢弃通知: %s", event.Domain)
	}
}

// RecordDomainQuery 记录域名开始查询的时间
func (a *NotificationAggregator) RecordDomainQuery(domain string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.lastDomainQueryMap[domain] = time.Now()
	a.lastQueryTime = time.Now()
}

// run 主循环
func (a *NotificationAggregator) run() {
	for {
		select {
		case event := <-a.eventCh:
			a.handleEvent(event)
		case <-a.stopCh:
			// 停止前发送所有待发送的通知
			a.sendPendingNotifications()
			return
		}
	}
}

// handleEvent 处理单个事件
func (a *NotificationAggregator) handleEvent(event NotificationEvent) {
	a.mu.Lock()

	// 检查是否为首次查询（没有旧状态）
	if event.OldStatus == "" || event.OldStatus == "unknown" {
		a.mu.Unlock()
		logger.Info("域名 %s 首次查询，状态为 %s，不发送通知", event.Domain, event.Status)
		return
	}

	// 检查是否状态没有变化（旧状态和新状态相同）
	if event.OldStatus == event.Status {
		a.mu.Unlock()
		logger.Debug("域名 %s 状态未变化 (%s)，跳过通知", event.Domain, event.Status)
		return
	}

	// 检查是否已为此次状态变化发送过通知（从旧状态到新状态）
	lastNotif, err := storage.GetLastNotification(event.Domain)
	if err != nil {
		logger.Error("获取最后通知记录失败: %v", err)
	} else if lastNotif != nil {
		// 如果最后一次通知的新状态和当前状态相同，说明已经通知过这个状态了
		if lastNotif.Status == event.Status && lastNotif.OldStatus == string(event.OldStatus) {
			a.mu.Unlock()
			logger.Info("域名 %s 状态变化 %s -> %s 已通知过，跳过", event.Domain, event.OldStatus, event.Status)
			return
		}
	}

	// 如果是第一个事件，创建通知组并启动10秒计时器
	if len(a.pendingEvents) == 0 {
		a.pendingEvents = append(a.pendingEvents, event)
		logger.Info("创建新的通知组，域名: %s, 状态变化: %s -> %s", event.Domain, event.OldStatus, event.Status)

		// 启动10秒计时器
		a.groupTimer = time.AfterFunc(10*time.Second, func() {
			a.sendPendingNotifications()
		})

		// 同时启动8秒无新查询检测
		go a.checkNoNewQuery(event.Timestamp)
		a.mu.Unlock()
		return
	}

	// 检查该域名是否已在待发送列表中
	isDuplicate := false
	for _, e := range a.pendingEvents {
		if e.Domain == event.Domain {
			isDuplicate = true
			break
		}
	}

	if !isDuplicate {
		a.pendingEvents = append(a.pendingEvents, event)
		logger.Info("域名 %s 加入当前通知组，状态变化: %s -> %s，当前组内域名数: %d",
			event.Domain, event.OldStatus, event.Status, len(a.pendingEvents))
	} else {
		logger.Debug("域名 %s 已在通知组中，跳过", event.Domain)
	}

	a.mu.Unlock()
}

// checkNoNewQuery 检查8秒内是否有新的查询
func (a *NotificationAggregator) checkNoNewQuery(groupStartTime time.Time) {
	time.Sleep(8 * time.Second)

	a.mu.Lock()
	defer a.mu.Unlock()

	// 如果已经没有待发送的事件，说明已经发送过了
	if len(a.pendingEvents) == 0 {
		return
	}

	// 检查最后一次查询时间是否在8秒之前
	timeSinceLastQuery := time.Since(a.lastQueryTime)
	if timeSinceLastQuery >= 8*time.Second {
		logger.Info("8秒内无新域名查询，立即发送通知组（包含 %d 个域名）", len(a.pendingEvents))

		// 取消10秒计时器
		if a.groupTimer != nil {
			a.groupTimer.Stop()
		}

		// 立即发送
		a.sendPendingNotificationsLocked()
	}
}

// sendPendingNotifications 发送所有待发送的通知（带锁）
func (a *NotificationAggregator) sendPendingNotifications() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.sendPendingNotificationsLocked()
}

// sendPendingNotificationsLocked 发送所有待发送的通知（无锁，内部使用）
func (a *NotificationAggregator) sendPendingNotificationsLocked() {
	if len(a.pendingEvents) == 0 {
		return
	}

	logger.Info("发送通知组，包含 %d 个域名状态变化", len(a.pendingEvents))

	// 如果只有一个事件，直接发送
	if len(a.pendingEvents) == 1 {
		event := a.pendingEvents[0]

		// 保存到通知历史
		if err := storage.SaveNotification(event.Domain, event.Status, event.OldStatus); err != nil {
			logger.Error("保存通知历史失败: %v", err)
		}

		// 发送通知
		a.mgr.SendNotificationDirect(event)
	} else {
		// 多个事件，合并发送
		a.sendBatchNotification()
	}

	// 清空待发送列表
	a.pendingEvents = make([]NotificationEvent, 0)

	// 停止计时器
	if a.groupTimer != nil {
		a.groupTimer.Stop()
		a.groupTimer = nil
	}
}

// sendBatchNotification 发送批量通知（合并多个域名状态变化）
func (a *NotificationAggregator) sendBatchNotification() {
	if len(a.pendingEvents) == 0 {
		return
	}

	// 保存所有通知历史
	for _, event := range a.pendingEvents {
		if err := storage.SaveNotification(event.Domain, event.Status, event.OldStatus); err != nil {
			logger.Error("保存通知历史失败: %v", err)
		}
	}

	// 发送合并通知
	a.mgr.SendNotificationDirectBatch(a.pendingEvents)
}
