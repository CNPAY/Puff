package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"Puff/logger"

	_ "github.com/glebarez/sqlite"
)

var (
	db     *sql.DB
	dbOnce sync.Once
	dbErr  error
)

// DomainEntry 表示存储在数据库中的域名记录
type DomainEntry struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Enabled   bool      `json:"enabled"`
	Notify    bool      `json:"notify"`
	CreatedAt time.Time `json:"created_at"`
}

// GetDB 返回全局数据库连接，确保只初始化一次
func GetDB() (*sql.DB, error) {
	dbOnce.Do(func() {
		// 确保存储目录存在
		if err := os.MkdirAll("data", 0755); err != nil {
			dbErr = fmt.Errorf("创建数据目录失败: %w", err)
			return
		}

		dbPath := filepath.Join("data", "puff.db")
		db, dbErr = sql.Open("sqlite", dbPath)
		if dbErr != nil {
			dbErr = fmt.Errorf("打开数据库失败: %w", dbErr)
			return
		}

		// 限制连接数，避免SQLite锁冲突
		db.SetMaxOpenConns(1)
		db.SetMaxIdleConns(1)

		if err := initSchema(db); err != nil {
			dbErr = err
			return
		}
	})

	return db, dbErr
}

// initSchema 初始化数据库表
func initSchema(db *sql.DB) error {
	schema := `
CREATE TABLE IF NOT EXISTS app_settings (
	key TEXT PRIMARY KEY,
	value TEXT
);

CREATE TABLE IF NOT EXISTS domains (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT UNIQUE NOT NULL,
	enabled INTEGER NOT NULL DEFAULT 1,
	notify INTEGER NOT NULL DEFAULT 1,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS domain_results (
	domain TEXT PRIMARY KEY,
	status TEXT,
	registrar TEXT,
	last_checked DATETIME,
	query_method TEXT,
	created_at DATETIME,
	expiry_at DATETIME,
	updated_at DATETIME,
	name_servers TEXT,
	whois_raw TEXT,
	error_message TEXT,
	created_at_record DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS notification_history (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	domain TEXT NOT NULL,
	status TEXT NOT NULL,
	old_status TEXT,
	sent_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	notification_type TEXT DEFAULT 'status_change',
	UNIQUE(domain, status)
);
`
	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("初始化数据库表失败: %w", err)
	}
	if err := ensureDomainResultColumns(db); err != nil {
		return err
	}
	return nil
}

// ensureDomainResultColumns 确保 domain_results 拥有新增列（迁移兼容）
func ensureDomainResultColumns(db *sql.DB) error {
	required := map[string]string{
		"created_at":        "ALTER TABLE domain_results ADD COLUMN created_at DATETIME",
		"expiry_at":         "ALTER TABLE domain_results ADD COLUMN expiry_at DATETIME",
		"updated_at":        "ALTER TABLE domain_results ADD COLUMN updated_at DATETIME",
		"name_servers":      "ALTER TABLE domain_results ADD COLUMN name_servers TEXT",
		"whois_raw":         "ALTER TABLE domain_results ADD COLUMN whois_raw TEXT",
		"error_message":     "ALTER TABLE domain_results ADD COLUMN error_message TEXT",
		"created_at_record": "ALTER TABLE domain_results ADD COLUMN created_at_record DATETIME DEFAULT CURRENT_TIMESTAMP",
	}
	existing := make(map[string]bool)
	rows, err := db.Query(`PRAGMA table_info(domain_results)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return err
		}
		existing[name] = true
	}
	for col, alter := range required {
		if !existing[col] {
			_, _ = db.Exec(alter) // 忽略已有列错误
		}
	}
	return nil
}

// UpsertSettings 批量写入设置
func UpsertSettings(settings map[string]string) error {
	db, err := GetDB()
	if err != nil {
		return err
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT INTO app_settings(key, value) VALUES(?, ?)
ON CONFLICT(key) DO UPDATE SET value=excluded.value`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for k, v := range settings {
		if _, err := stmt.Exec(k, v); err != nil {
			return fmt.Errorf("写入设置失败(%s): %w", k, err)
		}
	}

	return tx.Commit()
}

// GetAllSettings 读取所有设置键值
func GetAllSettings() (map[string]string, error) {
	db, err := GetDB()
	if err != nil {
		return nil, err
	}

	rows, err := db.Query(`SELECT key, value FROM app_settings`)
	if err != nil {
		return nil, fmt.Errorf("查询设置失败: %w", err)
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		result[k] = v
	}

	return result, rows.Err()
}

// GetSetting 获取单个设置
func GetSetting(key string) (string, bool, error) {
	db, err := GetDB()
	if err != nil {
		return "", false, err
	}

	var value string
	err = db.QueryRow(`SELECT value FROM app_settings WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("读取设置失败(%s): %w", key, err)
	}

	return value, true, nil
}

// ListDomains 返回域名列表，enabledOnly 为 true 时仅返回启用的域名
func ListDomains(enabledOnly bool) ([]DomainEntry, error) {
	db, err := GetDB()
	if err != nil {
		return nil, err
	}

	query := `SELECT id, name, enabled, notify, created_at FROM domains`
	if enabledOnly {
		query += ` WHERE enabled = 1`
	}
	query += ` ORDER BY id ASC`

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("查询域名失败: %w", err)
	}
	defer rows.Close()

	var domains []DomainEntry
	for rows.Next() {
		var d DomainEntry
		var enabledInt, notifyInt int
		if err := rows.Scan(&d.ID, &d.Name, &enabledInt, &notifyInt, &d.CreatedAt); err != nil {
			return nil, err
		}
		d.Enabled = enabledInt == 1
		d.Notify = notifyInt == 1
		domains = append(domains, d)
	}

	return domains, rows.Err()
}

// AddDomain 新增域名
func AddDomain(name string, enabled bool, notify bool) error {
	db, err := GetDB()
	if err != nil {
		return err
	}

	name = strings.TrimSpace(strings.ToLower(name))
	if name == "" {
		return fmt.Errorf("域名不能为空")
	}

	_, err = db.Exec(`INSERT INTO domains(name, enabled, notify) VALUES(?, ?, ?)
ON CONFLICT(name) DO UPDATE SET enabled=excluded.enabled, notify=excluded.notify`,
		name, boolToInt(enabled), boolToInt(notify))
	if err != nil {
		return fmt.Errorf("写入域名失败: %w", err)
	}
	return nil
}

// RemoveDomain 删除域名及所有相关数据
func RemoveDomain(name string) error {
	db, err := GetDB()
	if err != nil {
		return err
	}

	name = strings.TrimSpace(strings.ToLower(name))
	if name == "" {
		return fmt.Errorf("域名不能为空")
	}

	// 开启事务，确保所有删除操作原子性
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}
	defer tx.Rollback()

	// 1. 删除域名记录
	result, err := tx.Exec(`DELETE FROM domains WHERE name = ?`, name)
	if err != nil {
		return fmt.Errorf("删除域名失败: %w", err)
	}
	rowsAffected, _ := result.RowsAffected()
	logger.Debug("删除域名 %s: domains表删除了 %d 行", name, rowsAffected)

	// 2. 删除域名查询结果
	result, err = tx.Exec(`DELETE FROM domain_results WHERE domain = ?`, name)
	if err != nil {
		return fmt.Errorf("删除域名结果失败: %w", err)
	}
	rowsAffected, _ = result.RowsAffected()
	logger.Debug("删除域名 %s: domain_results表删除了 %d 行", name, rowsAffected)

	// 3. 删除通知历史记录
	result, err = tx.Exec(`DELETE FROM notification_history WHERE domain = ?`, name)
	if err != nil {
		return fmt.Errorf("删除通知历史失败: %w", err)
	}
	rowsAffected, _ = result.RowsAffected()
	logger.Debug("删除域名 %s: notification_history表删除了 %d 行", name, rowsAffected)

	// 提交事务
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交事务失败: %w", err)
	}

	logger.Info("成功删除域名 %s 及其所有相关数据", name)
	return nil
}

// RemoveDomains 批量删除域名及所有相关数据（优化版本，使用单个事务）
func RemoveDomains(names []string) error {
	if len(names) == 0 {
		return nil
	}

	db, err := GetDB()
	if err != nil {
		return err
	}

	// 标准化所有域名
	normalizedNames := make([]string, 0, len(names))
	for _, name := range names {
		name = strings.TrimSpace(strings.ToLower(name))
		if name != "" {
			normalizedNames = append(normalizedNames, name)
		}
	}

	if len(normalizedNames) == 0 {
		return fmt.Errorf("没有有效的域名需要删除")
	}

	// 开启事务，确保所有删除操作原子性
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}
	defer tx.Rollback()

	// 构建IN子句的占位符
	placeholders := make([]string, len(normalizedNames))
	args := make([]interface{}, len(normalizedNames))
	for i, name := range normalizedNames {
		placeholders[i] = "?"
		args[i] = name
	}
	inClause := strings.Join(placeholders, ",")

	// 1. 批量删除域名记录
	query := fmt.Sprintf(`DELETE FROM domains WHERE name IN (%s)`, inClause)
	result, err := tx.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("批量删除域名失败: %w", err)
	}
	domainsDeleted, _ := result.RowsAffected()

	// 2. 批量删除域名查询结果
	query = fmt.Sprintf(`DELETE FROM domain_results WHERE domain IN (%s)`, inClause)
	result, err = tx.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("批量删除域名结果失败: %w", err)
	}
	resultsDeleted, _ := result.RowsAffected()

	// 3. 批量删除通知历史记录
	query = fmt.Sprintf(`DELETE FROM notification_history WHERE domain IN (%s)`, inClause)
	result, err = tx.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("批量删除通知历史失败: %w", err)
	}
	historyDeleted, _ := result.RowsAffected()

	// 提交事务
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交事务失败: %w", err)
	}

	logger.Info("批量删除完成: 删除了 %d 个域名, %d 条查询结果, %d 条通知历史", 
		domainsDeleted, resultsDeleted, historyDeleted)
	return nil
}

// CleanOrphanedData 清理孤立数据（清理domain_results和notification_history中不在domains表中的数据）
func CleanOrphanedData() error {
	db, err := GetDB()
	if err != nil {
		return err
	}

	// 开启事务
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}
	defer tx.Rollback()

	// 1. 清理domain_results中的孤立数据
	result, err := tx.Exec(`
		DELETE FROM domain_results 
		WHERE domain NOT IN (SELECT name FROM domains)
	`)
	if err != nil {
		return fmt.Errorf("清理孤立查询结果失败: %w", err)
	}
	resultsDeleted, _ := result.RowsAffected()

	// 2. 清理notification_history中的孤立数据
	result, err = tx.Exec(`
		DELETE FROM notification_history 
		WHERE domain NOT IN (SELECT name FROM domains)
	`)
	if err != nil {
		return fmt.Errorf("清理孤立通知历史失败: %w", err)
	}
	historyDeleted, _ := result.RowsAffected()

	// 提交事务
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交事务失败: %w", err)
	}

	if resultsDeleted > 0 || historyDeleted > 0 {
		logger.Info("清理孤立数据完成: 删除了 %d 条查询结果, %d 条通知历史", 
			resultsDeleted, historyDeleted)
	} else {
		logger.Debug("未发现孤立数据")
	}

	return nil
}

// boolToInt 布尔转整型（SQLite用0/1）
func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

// DomainResult 表示域名查询结果
type DomainResult struct {
	Domain       string
	Status       string
	Registrar    string
	LastChecked  time.Time
	QueryMethod  string
	CreatedAt    *time.Time
	ExpiryAt     *time.Time
	UpdatedAt    *time.Time
	NameServers  []string
	WhoisRaw     string
	ErrorMessage string
}

// SaveDomainResult 保存单个域名查询结果
func SaveDomainResult(res DomainResult) error {
	db, err := GetDB()
	if err != nil {
		return err
	}
	_, err = db.Exec(`INSERT INTO domain_results(domain, status, registrar, last_checked, query_method, created_at, expiry_at, updated_at, name_servers, whois_raw, error_message)
VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(domain) DO UPDATE SET status=excluded.status, registrar=excluded.registrar, last_checked=excluded.last_checked, query_method=excluded.query_method, created_at=excluded.created_at, expiry_at=excluded.expiry_at, updated_at=excluded.updated_at, name_servers=excluded.name_servers, whois_raw=excluded.whois_raw, error_message=excluded.error_message`,
		res.Domain, res.Status, res.Registrar, res.LastChecked, res.QueryMethod, res.CreatedAt, res.ExpiryAt, res.UpdatedAt, strings.Join(res.NameServers, ","), res.WhoisRaw, res.ErrorMessage)
	return err
}

// LoadDomainResults 读取所有域名结果
func LoadDomainResults() (map[string]DomainResult, error) {
	db, err := GetDB()
	if err != nil {
		return nil, err
	}
	rows, err := db.Query(`SELECT domain, status, registrar, last_checked, query_method, created_at, expiry_at, updated_at, name_servers, COALESCE(whois_raw, ''), COALESCE(error_message, '') FROM domain_results ORDER BY domain ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]DomainResult)
	for rows.Next() {
		var r DomainResult
		var ns string
		var c, e, u sql.NullTime
		if err := rows.Scan(&r.Domain, &r.Status, &r.Registrar, &r.LastChecked, &r.QueryMethod, &c, &e, &u, &ns, &r.WhoisRaw, &r.ErrorMessage); err != nil {
			return nil, err
		}
		if c.Valid {
			r.CreatedAt = &c.Time
		}
		if e.Valid {
			r.ExpiryAt = &e.Time
		}
		if u.Valid {
			r.UpdatedAt = &u.Time
		}
		if strings.TrimSpace(ns) != "" {
			r.NameServers = strings.Split(ns, ",")
		}
		result[r.Domain] = r
	}
	return result, rows.Err()
}

// GetDomainResult 读取单个域名的查询结果
func GetDomainResult(domain string) (*DomainResult, error) {
	db, err := GetDB()
	if err != nil {
		return nil, err
	}
	
	domain = strings.ToLower(strings.TrimSpace(domain))
	
	var r DomainResult
	var ns string
	var c, e, u sql.NullTime
	
	err = db.QueryRow(`SELECT domain, status, registrar, last_checked, query_method, created_at, expiry_at, updated_at, name_servers, COALESCE(whois_raw, ''), COALESCE(error_message, '') FROM domain_results WHERE domain = ?`, domain).Scan(
		&r.Domain, &r.Status, &r.Registrar, &r.LastChecked, &r.QueryMethod, &c, &e, &u, &ns, &r.WhoisRaw, &r.ErrorMessage,
	)
	
	if err == sql.ErrNoRows {
		return nil, nil // 没有记录
	}
	if err != nil {
		return nil, fmt.Errorf("查询域名结果失败: %w", err)
	}
	
	if c.Valid {
		r.CreatedAt = &c.Time
	}
	if e.Valid {
		r.ExpiryAt = &e.Time
	}
	if u.Valid {
		r.UpdatedAt = &u.Time
	}
	if strings.TrimSpace(ns) != "" {
		r.NameServers = strings.Split(ns, ",")
	}
	
	return &r, nil
}

// UpdateWhoisRaw 更新域名的原始WHOIS数据
func UpdateWhoisRaw(domain, whoisRaw string) error {
	db, err := GetDB()
	if err != nil {
		return err
	}

	domain = strings.ToLower(strings.TrimSpace(domain))
	if domain == "" {
		return fmt.Errorf("域名不能为空")
	}

	_, err = db.Exec(`UPDATE domain_results SET whois_raw = ? WHERE domain = ?`, whoisRaw, domain)
	if err != nil {
		return fmt.Errorf("更新WHOIS原始数据失败: %w", err)
	}

	return nil
}

// NotificationRecord 通知记录
type NotificationRecord struct {
	ID               int64
	Domain           string
	Status           string
	OldStatus        string
	SentAt           time.Time
	NotificationType string
}

// GetLastNotification 获取域名的最后一次通知记录
func GetLastNotification(domain string) (*NotificationRecord, error) {
	db, err := GetDB()
	if err != nil {
		return nil, err
	}

	domain = strings.ToLower(strings.TrimSpace(domain))

	var record NotificationRecord
	err = db.QueryRow(`SELECT id, domain, status, COALESCE(old_status, ''), sent_at, notification_type 
		FROM notification_history WHERE domain = ? ORDER BY sent_at DESC LIMIT 1`, domain).Scan(
		&record.ID, &record.Domain, &record.Status, &record.OldStatus, &record.SentAt, &record.NotificationType,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询通知记录失败: %w", err)
	}

	return &record, nil
}

// SaveNotification 保存通知记录
func SaveNotification(domain, status, oldStatus string) error {
	db, err := GetDB()
	if err != nil {
		return err
	}

	domain = strings.ToLower(strings.TrimSpace(domain))
	if domain == "" {
		return fmt.Errorf("域名不能为空")
	}

	_, err = db.Exec(`INSERT INTO notification_history(domain, status, old_status, notification_type) 
		VALUES(?, ?, ?, 'status_change')
		ON CONFLICT(domain, status) DO UPDATE SET sent_at = CURRENT_TIMESTAMP, old_status = excluded.old_status`,
		domain, status, oldStatus)

	if err != nil {
		return fmt.Errorf("保存通知记录失败: %w", err)
	}

	return nil
}

// HasNotifiedForStatus 检查是否已为此状态发送过通知
func HasNotifiedForStatus(domain, status string) (bool, error) {
	db, err := GetDB()
	if err != nil {
		return false, err
	}

	domain = strings.ToLower(strings.TrimSpace(domain))

	var count int
	err = db.QueryRow(`SELECT COUNT(*) FROM notification_history WHERE domain = ? AND status = ?`,
		domain, status).Scan(&count)

	if err != nil {
		return false, fmt.Errorf("查询通知记录失败: %w", err)
	}

	return count > 0, nil
}
