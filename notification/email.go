package notification

import (
	"crypto/tls"
	"fmt"
	"net/smtp"
	"strings"

	"Puff/config"
)

// translateStatus 翻译状态为中文
func translateStatus(status string) string {
	statusMap := map[string]string{
		"available":      "可注册",
		"registered":     "已注册",
		"grace":          "宽限期",
		"redemption":     "赎回期",
		"pending_delete": "待删除",
		"expired":        "已过期",
		"error":          "查询失败",
		"unknown":        "未知",
	}

	if chinese, ok := statusMap[status]; ok {
		return chinese
	}
	return status
}

// translateStatusChange 翻译状态变化为中文
func translateStatusChange(oldStatus, newStatus string) string {
	return translateStatus(oldStatus) + " → " + translateStatus(newStatus)
}

// EmailNotifier 邮件通知器
type EmailNotifier struct {
	config  config.SMTPConfig
	enabled bool
}

// NewEmailNotifier 创建邮件通知器
func NewEmailNotifier(cfg config.SMTPConfig) *EmailNotifier {
	return &EmailNotifier{
		config:  cfg,
		enabled: cfg.Enabled,
	}
}

// SendMessage 发送邮件
func (e *EmailNotifier) SendMessage(subject, message string) error {
	if !e.enabled {
		return fmt.Errorf("邮件通知未启用")
	}

	// 验证配置
	if err := e.validateConfig(); err != nil {
		return fmt.Errorf("邮件配置无效: %v", err)
	}

	// 构建邮件内容
	body := e.buildEmailBody(subject, message)

	// 发送邮件
	return e.sendEmail(subject, body)
}

// IsEnabled 检查是否启用
func (e *EmailNotifier) IsEnabled() bool {
	return e.enabled && e.config.Enabled
}

// UpdateConfig 更新配置
func (e *EmailNotifier) UpdateConfig(cfg config.SMTPConfig) {
	e.config = cfg
	e.enabled = cfg.Enabled
}

// GetType 获取通知器类型
func (e *EmailNotifier) GetType() string {
	return "email"
}

// Test 测试邮件连接
func (e *EmailNotifier) Test() error {
	if !e.enabled {
		return fmt.Errorf("邮件通知未启用")
	}

	// 验证配置
	if err := e.validateConfig(); err != nil {
		return err
	}

	// 发送测试邮件
	subject := "测试邮件"
	message := "这是一封测试邮件，用于验证 Puff 邮件通知功能是否正常工作。\n\n如果您收到这封邮件，说明邮件通知配置正确。"

	err := e.SendMessage(subject, message)

	// 特殊处理：如果错误包含"short response"，认为发送成功
	// 这是因为某些SMTP服务器在发送成功后会返回不完整的响应
	if err != nil && strings.Contains(err.Error(), "short response") {
		return nil // 视为成功
	}

	return err
}

// validateConfig 验证配置
func (e *EmailNotifier) validateConfig() error {
	if e.config.Host == "" {
		return fmt.Errorf("SMTP服务器地址不能为空")
	}

	if e.config.Port <= 0 || e.config.Port > 65535 {
		return fmt.Errorf("SMTP端口无效: %d", e.config.Port)
	}

	if e.config.User == "" {
		return fmt.Errorf("SMTP用户名不能为空")
	}

	if e.config.Password == "" {
		return fmt.Errorf("SMTP密码不能为空")
	}

	if e.config.From == "" {
		return fmt.Errorf("发件人地址不能为空")
	}

	if e.config.To == "" {
		return fmt.Errorf("收件人地址不能为空")
	}

	return nil
}

// buildEmailBody 构建邮件内容
func (e *EmailNotifier) buildEmailBody(subject, message string) string {
	var body strings.Builder

	// 邮件头
	body.WriteString(fmt.Sprintf("From: %s\r\n", e.config.From))
	body.WriteString(fmt.Sprintf("To: %s\r\n", e.config.To))
	body.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	body.WriteString("MIME-Version: 1.0\r\n")
	body.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	body.WriteString("Content-Transfer-Encoding: 8bit\r\n")
	body.WriteString("\r\n")

	// 邮件正文 - HTML格式
	body.WriteString(e.buildHTMLContent(subject, message))

	return body.String()
}

// buildHTMLContent 构建HTML邮件内容（DaisyUI Lofi风格）
func (e *EmailNotifier) buildHTMLContent(subject, message string) string {
	// 检查是否为批量通知
	if strings.Contains(message, "检测到") && strings.Contains(message, "个域名状态发生变化") {
		return e.buildBatchHTMLContent(subject, message)
	}

	// 解析消息内容
	lines := strings.Split(message, "\n")
	domain := ""
	timestamp := ""
	oldStatus := ""
	newStatus := ""
	statusInfo := ""

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "域名: ") {
			domain = strings.TrimPrefix(line, "域名: ")
		} else if strings.HasPrefix(line, "时间: ") {
			timestamp = strings.TrimPrefix(line, "时间: ")
		} else if strings.HasPrefix(line, "状态变化: ") {
			statusChangeStr := strings.TrimPrefix(line, "状态变化: ")
			// 解析 "old_status → new_status"
			parts := strings.Split(statusChangeStr, "→")
			if len(parts) == 2 {
				oldStatus = strings.TrimSpace(parts[0])
				newStatus = strings.TrimSpace(parts[1])
			}
		} else if strings.HasPrefix(line, "状态: ") {
			statusInfo = strings.TrimPrefix(line, "状态: ")
		}
	}

	// DaisyUI Lofi风格: 扁平、黑白灰、极简
	html := `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        body {
            font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
            background-color: #fafafa;
            padding: 40px 20px;
            line-height: 1.6;
        }
        .container {
            max-width: 600px;
            margin: 0 auto;
            background-color: #ffffff;
            border: 2px solid #000000;
            box-shadow: 4px 4px 0 0 #000000;
        }
        .header {
            background-color: #000000;
            color: #ffffff;
            padding: 20px;
            border-bottom: 2px solid #000000;
        }
        .header h1 {
            font-size: 18px;
            font-weight: 700;
            letter-spacing: 0.5px;
            text-transform: uppercase;
            color: #ffffff !important;
        }
        .content {
            padding: 30px 20px;
        }
        .info-box {
            border: 2px solid #000000;
            padding: 16px;
            margin-bottom: 16px;
            background-color: #ffffff;
        }
        .info-label {
            font-size: 11px;
            font-weight: 700;
            text-transform: uppercase;
            letter-spacing: 1px;
            color: #666666;
            margin-bottom: 6px;
        }
        .info-value {
            font-size: 16px;
            font-weight: 600;
            color: #000000;
            word-break: break-all;
        }
        .status-change-box {
            border: 2px solid #000000;
            background-color: #f5f5f5;
            padding: 20px;
            text-align: center;
            margin: 24px 0;
        }
        .status-arrow {
            display: flex;
            align-items: center;
            justify-content: center;
            gap: 12px;
            font-size: 16px;
            font-weight: 700;
        }
        .status-old {
            background-color: #ffffff;
            border: 2px solid #000000;
            padding: 8px 16px;
        }
        .status-new {
            background-color: #000000;
            color: #ffffff;
            border: 2px solid #000000;
            padding: 8px 16px;
        }
        .arrow {
            font-size: 20px;
            font-weight: 700;
        }
        .footer {
            background-color: #f5f5f5;
            padding: 16px 20px;
            border-top: 2px solid #000000;
            text-align: center;
            font-size: 11px;
            color: #666666;
            text-transform: uppercase;
            letter-spacing: 0.5px;
        }
        .domain-name {
            font-size: 20px;
            font-weight: 700;
            color: #000000!important;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1><a style="color: #ffffff !important;">` + subject + `</a></h1>
        </div>
        <div class="content">`

	if domain != "" {
		html += `
            <div class="info-box">
                <div class="info-label">Domain</div>
                <div class="info-value domain-name"><a style="color: #000000 !important;">` + domain + `</a></div>
            </div>`
	}

	if timestamp != "" {
		html += `
            <div class="info-box">
                <div class="info-label">Time</div>
                <div class="info-value">` + timestamp + `</div>
            </div>`
	}

	// 状态变化（翻译成中文）
	if oldStatus != "" && newStatus != "" {
		translatedOld := translateStatus(oldStatus)
		translatedNew := translateStatus(newStatus)
		html += `
            <div class="status-change-box">
                <div class="status-arrow">
                    <div class="status-old">` + translatedOld + `</div>
                    <div class="arrow">→</div>
                    <div class="status-new">` + translatedNew + `</div>
                </div>
            </div>`
	} else if statusInfo != "" {
		html += `
            <div class="info-box">
                <div class="info-label">Status</div>
                <div class="info-value">` + statusInfo + `</div>
            </div>`
	}

	html += `
        </div>
        <div class="footer">
            Puff Domain Monitor System<br>
            Auto-generated - Do not reply
        </div>
    </div>
</body>
</html>`

	return html
}

// buildBatchHTMLContent 构建批量通知的HTML内容
func (e *EmailNotifier) buildBatchHTMLContent(subject, message string) string {
	// 解析消息内容
	lines := strings.Split(message, "\n")
	timestamp := ""
	domainChanges := []struct {
		Domain    string
		OldStatus string
		NewStatus string
	}{}

	// 解析时间和域名变化
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "时间: ") {
			timestamp = strings.TrimPrefix(line, "时间: ")
		} else if strings.Contains(line, ". ") && !strings.HasPrefix(line, "---") {
			// 解析域名行 "1. test1.puff"
			parts := strings.SplitN(line, ". ", 2)
			if len(parts) == 2 {
				domain := parts[1]
				// 下一行应该是状态变化
				if i+1 < len(lines) {
					nextLine := strings.TrimSpace(lines[i+1])
					if strings.HasPrefix(nextLine, "状态变化: ") {
						statusChangeStr := strings.TrimPrefix(nextLine, "状态变化: ")
						statusParts := strings.Split(statusChangeStr, "→")
						if len(statusParts) == 2 {
							domainChanges = append(domainChanges, struct {
								Domain    string
								OldStatus string
								NewStatus string
							}{
								Domain:    domain,
								OldStatus: strings.TrimSpace(statusParts[0]),
								NewStatus: strings.TrimSpace(statusParts[1]),
							})
						}
					}
				}
			}
		}
	}

	// 构建HTML
	html := `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        body {
            font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
            background-color: #fafafa;
            padding: 40px 20px;
            line-height: 1.6;
        }
        .container {
            max-width: 600px;
            margin: 0 auto;
            background-color: #ffffff;
            border: 2px solid #000000;
            box-shadow: 4px 4px 0 0 #000000;
        }
        .header {
            background-color: #000000;
            color: #ffffff;
            padding: 20px;
            border-bottom: 2px solid #000000;
        }
        .header h1 {
            font-size: 18px;
            font-weight: 700;
            letter-spacing: 0.5px;
            text-transform: uppercase;
            color: #ffffff;
        }
        .content {
            padding: 30px 20px;
        }
        .info-box {
            border: 2px solid #000000;
            padding: 16px;
            margin-bottom: 16px;
            background-color: #ffffff;
        }
        .info-label {
            font-size: 11px;
            font-weight: 700;
            text-transform: uppercase;
            letter-spacing: 1px;
            color: #666666;
            margin-bottom: 6px;
        }
        .info-value {
            font-size: 16px;
            font-weight: 600;
            color: #000000;
        }
        .domain-item {
            border: 2px solid #000000;
            padding: 12px;
            margin-bottom: 12px;
            background-color: #f5f5f5;
        }
        .domain-name {
            font-size: 16px;
            font-weight: 700;
            color: #000000;
            margin-bottom: 8px;
        }
        .status-change {
            display: flex;
            align-items: center;
            gap: 8px;
            font-size: 14px;
            font-weight: 600;
        }
        .status-old {
            background-color: #ffffff;
            border: 1px solid #000000;
            padding: 4px 12px;
        }
        .status-new {
            background-color: #000000;
            color: #ffffff;
            border: 1px solid #000000;
            padding: 4px 12px;
        }
        .arrow {
            font-size: 16px;
            font-weight: 700;
        }
        .footer {
            background-color: #f5f5f5;
            padding: 16px 20px;
            border-top: 2px solid #000000;
            text-align: center;
            font-size: 11px;
            color: #666666;
            text-transform: uppercase;
            letter-spacing: 0.5px;
        }
        .summary {
            font-size: 14px;
            color: #666666;
            margin-bottom: 20px;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>` + subject + `</h1>
        </div>
        <div class="content">`

	if timestamp != "" {
		html += `
            <div class="info-box">
                <div class="info-label">Time</div>
                <div class="info-value">` + timestamp + `</div>
            </div>`
	}

	html += `
            <div class="summary">检测到 ` + fmt.Sprintf("%d", len(domainChanges)) + ` 个域名状态发生变化</div>`

	// 添加每个域名的变化
	for _, change := range domainChanges {
		translatedOld := translateStatus(change.OldStatus)
		translatedNew := translateStatus(change.NewStatus)
		html += `
            <div class="domain-item">
                <div class="domain-name"><a style="color: #000000 !important;">` + change.Domain + `</a></div>
                <div class="status-change">
                    <div class="status-old">` + translatedOld + `</div>
                    <div class="arrow">→</div>
                    <div class="status-new">` + translatedNew + `</div>
                </div>
            </div>`
	}

	html += `
        </div>
        <div class="footer">
            Puff Domain Monitor System<br>
            Auto-generated - Do not reply
        </div>
    </div>
</body>
</html>`

	return html
}

// sendEmail 发送邮件
func (e *EmailNotifier) sendEmail(subject, body string) error {
	// 构建服务器地址
	addr := fmt.Sprintf("%s:%d", e.config.Host, e.config.Port)

	// 创建认证
	auth := smtp.PlainAuth("", e.config.User, e.config.Password, e.config.Host)

	// 解析收件人地址
	recipients := e.parseRecipients(e.config.To)

	// 判断是否使用TLS
	if e.config.Port == 465 {
		// 使用SSL/TLS连接
		return e.sendWithTLS(addr, auth, recipients, body)
	} else {
		// 使用STARTTLS连接
		return e.sendWithSTARTTLS(addr, auth, recipients, body)
	}
}

// sendWithTLS 使用TLS发送邮件
func (e *EmailNotifier) sendWithTLS(addr string, auth smtp.Auth, recipients []string, body string) error {
	// 创建TLS连接
	tlsConfig := &tls.Config{
		ServerName: e.config.Host,
	}

	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("TLS连接失败: %v", err)
	}
	defer conn.Close()

	// 创建SMTP客户端
	client, err := smtp.NewClient(conn, e.config.Host)
	if err != nil {
		return fmt.Errorf("创建SMTP客户端失败: %v", err)
	}
	defer client.Close()

	// 认证
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("SMTP认证失败: %v", err)
	}

	// 设置发件人
	if err := client.Mail(e.config.From); err != nil {
		return fmt.Errorf("设置发件人失败: %v", err)
	}

	// 设置收件人
	for _, recipient := range recipients {
		if err := client.Rcpt(recipient); err != nil {
			return fmt.Errorf("设置收件人失败: %v", err)
		}
	}

	// 发送邮件内容
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("获取数据写入器失败: %v", err)
	}

	_, err = w.Write([]byte(body))
	if err != nil {
		return fmt.Errorf("写入邮件内容失败: %v", err)
	}

	err = w.Close()
	if err != nil {
		return fmt.Errorf("关闭数据写入器失败: %v", err)
	}

	return client.Quit()
}

// sendWithSTARTTLS 使用STARTTLS发送邮件
func (e *EmailNotifier) sendWithSTARTTLS(addr string, auth smtp.Auth, recipients []string, body string) error {
	return smtp.SendMail(addr, auth, e.config.From, recipients, []byte(body))
}

// parseRecipients 解析收件人地址
func (e *EmailNotifier) parseRecipients(to string) []string {
	// 支持多个收件人，用逗号分隔
	recipients := strings.Split(to, ",")

	// 清理空格
	for i, recipient := range recipients {
		recipients[i] = strings.TrimSpace(recipient)
	}

	return recipients
}

// SetEnabled 设置启用状态
func (e *EmailNotifier) SetEnabled(enabled bool) {
	e.enabled = enabled
}
