package notification

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"Puff/config"
)

// Notifier 通知器接口
type Notifier interface {
	// SendMessage 发送消息
	SendMessage(subject, message string) error

	// IsEnabled 检查是否启用
	IsEnabled() bool

	// GetType 获取通知器类型
	GetType() string

	// Test 测试连接
	Test() error
}

// NotificationEvent 通知事件
type NotificationEvent struct {
	Type      string    `json:"type"`       // 事件类型
	Domain    string    `json:"domain"`     // 域名
	Status    string    `json:"status"`     // 当前状态
	OldStatus string    `json:"old_status"` // 之前状态
	Message   string    `json:"message"`    // 消息内容
	Timestamp time.Time `json:"timestamp"`  // 时间戳
	WhoisRaw  string    `json:"whois_raw"`  // 原始Whois信息
}

// NotificationManager 通知管理器
type NotificationManager struct {
	notifiers   []Notifier
	queue       chan NotificationEvent
	enabled     bool
	sentHistory map[string]map[string]time.Time // domain -> status -> last_sent_time
	mu          sync.RWMutex
	aggregator  *NotificationAggregator // 通知聚合器
}

// NewNotificationManager 创建通知管理器
func NewNotificationManager() *NotificationManager {
	mgr := &NotificationManager{
		notifiers:   make([]Notifier, 0),
		queue:       make(chan NotificationEvent, 1000),
		enabled:     true,
		sentHistory: make(map[string]map[string]time.Time),
	}

	// 创建聚合器
	mgr.aggregator = NewNotificationAggregator(mgr)

	return mgr
}

// AddNotifier 添加通知器
func (nm *NotificationManager) AddNotifier(notifier Notifier) {
	// 始终添加通知器，启用状态由配置控制
	nm.notifiers = append(nm.notifiers, notifier)
}

// Start 启动通知管理器
func (nm *NotificationManager) Start() {
	go nm.processNotifications()
	// 启动聚合器
	if nm.aggregator != nil {
		nm.aggregator.Start()
	}
}

// Stop 停止通知管理器
func (nm *NotificationManager) Stop() {
	nm.enabled = false

	// 停止聚合器
	if nm.aggregator != nil {
		nm.aggregator.Stop()
	}

	close(nm.queue)
}

// SendNotification 发送通知（通过聚合器）
func (nm *NotificationManager) SendNotification(event NotificationEvent) {
	if !nm.enabled {
		return
	}

	// 状态变更事件通过聚合器处理
	if event.Type == "status_change" && nm.aggregator != nil {
		nm.aggregator.AddEvent(event)
		return
	}

	// 其他类型的事件直接发送
	nm.SendNotificationDirect(event)
}

// SendNotificationDirect 直接发送通知（绕过聚合器）
func (nm *NotificationManager) SendNotificationDirect(event NotificationEvent) {
	if !nm.enabled {
		return
	}

	select {
	case nm.queue <- event:
		// 成功加入队列
		if event.Type == "status_change" {
			nm.recordNotification(event.Domain, event.Status)
		}
	default:
		// 队列满了，丢弃消息
		log.Printf("通知队列已满，丢弃通知: %s", event.Domain)
	}
}

// SendNotificationDirectBatch 直接发送批量通知（绕过聚合器）
func (nm *NotificationManager) SendNotificationDirectBatch(events []NotificationEvent) {
	if !nm.enabled || len(events) == 0 {
		return
	}

	// 构建批量通知的主题和消息
	subject := nm.formatBatchSubject(events)
	message := nm.formatBatchMessage(events)

	// 发送给所有通知器
	for _, notifier := range nm.notifiers {
		if notifier.IsEnabled() {
			go func(n Notifier) {
				if err := n.SendMessage(subject, message); err != nil {
					// 记录错误日志
					if err != nil && strings.Contains(err.Error(), "short response") {
					} else {
						log.Printf("[错误] 发送 %s 批量通知失败: %v", n.GetType(), err)
					}
				} else {
					log.Printf("[成功] %s 批量通知发送成功，包含 %d 个域名", n.GetType(), len(events))
				}
			}(notifier)
		}
	}

	// 记录所有域名的通知历史
	for _, event := range events {
		nm.recordNotification(event.Domain, event.Status)
	}
}

// RecordDomainQuery 记录域名开始查询
func (nm *NotificationManager) RecordDomainQuery(domain string) {
	if nm.aggregator != nil {
		nm.aggregator.RecordDomainQuery(domain)
	}
}

// shouldSendNotification 检查是否应该发送通知（去重逻辑）
func (nm *NotificationManager) shouldSendNotification(domain, status string) bool {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	// 检查是否已经为此域名的此状态发送过通知
	if domainHistory, exists := nm.sentHistory[domain]; exists {
		if lastSent, statusExists := domainHistory[status]; statusExists {
			// 如果在过去24小时内已经发送过相同状态的通知，则跳过
			if time.Since(lastSent) < 24*time.Hour {
				return false
			}
		}
	}

	return true
}

// recordNotification 记录通知发送历史
func (nm *NotificationManager) recordNotification(domain, status string) {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	if nm.sentHistory[domain] == nil {
		nm.sentHistory[domain] = make(map[string]time.Time)
	}
	nm.sentHistory[domain][status] = time.Now()
}

// ClearHistory 清理过期的通知历史（超过30天）
func (nm *NotificationManager) ClearHistory() {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	cutoff := time.Now().Add(-30 * 24 * time.Hour)

	for domain, statusHistory := range nm.sentHistory {
		for status, lastSent := range statusHistory {
			if lastSent.Before(cutoff) {
				delete(statusHistory, status)
			}
		}

		// 如果域名没有任何记录了，删除整个域名条目
		if len(statusHistory) == 0 {
			delete(nm.sentHistory, domain)
		}
	}

	log.Printf("清理过期通知历史，当前跟踪 %d 个域名", len(nm.sentHistory))
}

// processNotifications 处理通知队列
func (nm *NotificationManager) processNotifications() {
	for event := range nm.queue {
		nm.sendToAllNotifiers(event)
	}
}

// sendToAllNotifiers 发送给所有通知器
func (nm *NotificationManager) sendToAllNotifiers(event NotificationEvent) {
	subject := nm.formatSubject(event)
	message := nm.formatMessage(event)

	for _, notifier := range nm.notifiers {
		if notifier.IsEnabled() {
			go func(n Notifier) {
				if err := n.SendMessage(subject, message); err != nil {
					// 记录错误日志
					if err != nil && strings.Contains(err.Error(), "short response") {
					} else {
						log.Printf("[错误] 发送 %s 通知失败: %v", n.GetType(), err)
					}
					log.Printf("[错误] 通知内容: 域名=%s, 事件类型=%s", event.Domain, event.Type)
				} else {
					log.Printf("[成功] %s 通知发送成功: 域名=%s", n.GetType(), event.Domain)
				}
			}(notifier)
		}
	}
}

// formatSubject 格式化主题
func (nm *NotificationManager) formatSubject(event NotificationEvent) string {
	switch event.Type {
	case "status_change":
		return fmt.Sprintf("%s 状态变化", event.Domain)
	case "available":
		return fmt.Sprintf("%s 可注册！", event.Domain)
	case "redemption":
		return fmt.Sprintf("%s 进入赎回期", event.Domain)
	case "pending_delete":
		return fmt.Sprintf("%s 进入待删除期", event.Domain)
	case "error":
		return fmt.Sprintf("%s 查询失败", event.Domain)
	default:
		return fmt.Sprintf("%s 通知", event.Domain)
	}
}

// formatMessage 格式化消息
func (nm *NotificationManager) formatMessage(event NotificationEvent) string {
	var message strings.Builder

	message.WriteString(fmt.Sprintf("域名: %s\n", event.Domain))
	message.WriteString(fmt.Sprintf("时间: %s\n", event.Timestamp.Format("2006-01-02 15:04:05")))

	switch event.Type {
	case "status_change":
		message.WriteString(fmt.Sprintf("状态变化: %s → %s\n", event.OldStatus, event.Status))
	case "available":
		message.WriteString("状态: 可注册\n")
		message.WriteString("此域名现在可以注册！\n")
	case "redemption":
		message.WriteString("状态: 赎回期\n")
		message.WriteString("此域名现在处于赎回期，可以尝试赎回。\n")
	case "pending_delete":
		message.WriteString("状态: 待删除\n")
		message.WriteString("此域名即将删除，进入抢注阶段！\n")
	case "error":
		message.WriteString("状态: 查询失败\n")
		message.WriteString(fmt.Sprintf("错误信息: %s\n", event.Message))
	}

	if event.Message != "" && event.Type != "error" {
		message.WriteString(fmt.Sprintf("\n详细信息: %s\n", event.Message))
	}

	if event.WhoisRaw != "" {
		message.WriteString("\n=== WHOIS/RDAP 信息 ===\n")
		// 限制长度以免消息过长
		if len(event.WhoisRaw) > 2000 {
			message.WriteString(event.WhoisRaw[:2000] + "\n...(已截断)")
		} else {
			message.WriteString(event.WhoisRaw)
		}
		message.WriteString("\n")
	}

	message.WriteString("\n---\n")
	message.WriteString("此消息由 Puff 自动发送")

	return message.String()
}

// formatBatchSubject 格式化批量通知主题
func (nm *NotificationManager) formatBatchSubject(events []NotificationEvent) string {
	return fmt.Sprintf("域名状态变化通知 (%d个域名)", len(events))
}

// formatBatchMessage 格式化批量通知消息
func (nm *NotificationManager) formatBatchMessage(events []NotificationEvent) string {
	var message strings.Builder

	message.WriteString(fmt.Sprintf("检测到 %d 个域名状态发生变化\n", len(events)))
	message.WriteString(fmt.Sprintf("时间: %s\n\n", time.Now().Format("2006-01-02 15:04:05")))

	// 列出所有域名的状态变化
	for i, event := range events {
		message.WriteString(fmt.Sprintf("%d. %s\n", i+1, event.Domain))
		message.WriteString(fmt.Sprintf("   状态变化: %s → %s\n", event.OldStatus, event.Status))
		if i < len(events)-1 {
			message.WriteString("\n")
		}
	}

	message.WriteString("\n---\n")
	message.WriteString("此消息由 Puff 自动发送")

	return message.String()
}

// TestAllNotifiers 测试所有通知器
func (nm *NotificationManager) TestAllNotifiers() map[string]error {
	results := make(map[string]error)

	for _, notifier := range nm.notifiers {
		results[notifier.GetType()] = notifier.Test()
	}

	return results
}

// GetEnabledNotifiers 获取启用的通知器列表
func (nm *NotificationManager) GetEnabledNotifiers() []string {
	var enabled []string

	for _, notifier := range nm.notifiers {
		if notifier.IsEnabled() {
			enabled = append(enabled, notifier.GetType())
		}
	}

	return enabled
}

// GetNotifiers 获取所有通知器
func (nm *NotificationManager) GetNotifiers() []Notifier {
	return nm.notifiers
}

// UpdateEmailConfig 更新邮件通知器配置
func (nm *NotificationManager) UpdateEmailConfig(cfg config.SMTPConfig) error {
	for _, notifier := range nm.notifiers {
		if notifier.GetType() == "email" {
			if emailNotifier, ok := notifier.(*EmailNotifier); ok {
				emailNotifier.UpdateConfig(cfg)
				return nil
			}
		}
	}
	return fmt.Errorf("未找到邮件通知器")
}

// UpdateTelegramConfig 更新Telegram通知器配置
func (nm *NotificationManager) UpdateTelegramConfig(cfg config.TelegramConfig) error {
	for _, notifier := range nm.notifiers {
		if notifier.GetType() == "telegram" {
			if telegramNotifier, ok := notifier.(*TelegramNotifier); ok {
				telegramNotifier.UpdateConfig(cfg)
				return nil
			}
		}
	}
	return fmt.Errorf("未找到Telegram通知器")
}

// GetStats 获取统计信息
func (nm *NotificationManager) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"enabled":           nm.enabled,
		"notifier_count":    len(nm.notifiers),
		"queue_length":      len(nm.queue),
		"queue_capacity":    cap(nm.queue),
		"enabled_notifiers": nm.GetEnabledNotifiers(),
	}
}
