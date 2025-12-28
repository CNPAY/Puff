// 全局变量
let domains = [];
let filteredDomains = [];
let currentPage = 'dashboard';
let totalDomains = 0;
let totalManageDomains = 0;

// 分页相关变量
let dashboardCurrentPage = 1;
let manageCurrentPage = 1;
let itemsPerPage = 20; // 默认每页20条

let currentSort = { field: 'name', order: 'asc' };

// DOM元素
const elements = {
    // 导航
    navDashboard: document.getElementById('nav-dashboard'),
    navDomains: document.getElementById('nav-domains'),
    navSettings: document.getElementById('nav-settings'),
    
    // 页面
    dashboardPage: document.getElementById('dashboard-page'),
    domainsPage: document.getElementById('domains-page'),
    settingsPage: document.getElementById('settings-page'),
    
    // 统计
    totalDomains: document.getElementById('totalDomains'),
    availableDomains: document.getElementById('availableDomains'),
    graceDomains: document.getElementById('graceDomains'),
    pendingDeleteDomains: document.getElementById('pendingDeleteDomains'),
    redemptionDomains: document.getElementById('redemptionDomains'),
    
    // 监控控制
    monitorStatus: document.getElementById('monitorStatus'),
    startMonitorBtn: document.getElementById('startMonitorBtn'),
    stopMonitorBtn: document.getElementById('stopMonitorBtn'),
    reloadBtn: document.getElementById('reloadBtn'),
    testNotificationBtn: document.getElementById('testNotificationBtn'),
    
    // 搜索
    searchInput: document.getElementById('searchInput'),
    searchBtn: document.getElementById('searchBtn'),
    statusFilter: document.getElementById('statusFilter'),
    
    // 表格
    domainTableBody: document.getElementById('domainTableBody'),
    selectAllCheckbox: document.getElementById('selectAllCheckbox'),
    checkSelectedBtn: document.getElementById('checkSelectedBtn'),
    deleteSelectedBtn: document.getElementById('deleteSelectedBtn'),
    
    // 域名管理
    singleDomainInput: document.getElementById('singleDomainInput'),
    addSingleDomainBtn: document.getElementById('addSingleDomainBtn'),
    batchDomainInput: document.getElementById('batchDomainInput'),
    addBatchDomainsBtn: document.getElementById('addBatchDomainsBtn'),
    manageDomainTableBody: document.getElementById('manageDomainTableBody'),
    
    // 其他
    refreshBtn: document.getElementById('refreshBtn'),
    logoutBtn: document.getElementById('logout-btn'),
    loadingIndicator: document.getElementById('loadingIndicator'),
    emptyState: document.getElementById('emptyState'),
    notifications: document.getElementById('notifications')
};

const dateTimeOptions = {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false
};

// 初始化
document.addEventListener('DOMContentLoaded', function() {
    initializeEventListeners();
    initializeNavigation();
    loadData();
    // 检查更新
    checkForUpdates();
    
    // 定期检查更新（每6小时）
    setInterval(checkForUpdates, 6 * 60 * 60 * 1000);
    
    setInterval(() => {
         if (currentPage === 'dashboard') {
             loadData();
         }
     }, 30000); // 每30秒刷新一次
});

// 初始化事件监听器
function initializeEventListeners() {
    // 导航
    elements.navDashboard?.addEventListener('click', () => switchPage('dashboard'));
    elements.navDomains?.addEventListener('click', () => switchPage('domains'));
    elements.navSettings?.addEventListener('click', () => switchPage('settings'));
    
    // 移动端导航
    document.getElementById('nav-dashboard-mobile')?.addEventListener('click', (e) => {
        e.preventDefault();
        switchPage('dashboard');
        closeMobileDropdown();
    });
    document.getElementById('nav-domains-mobile')?.addEventListener('click', (e) => {
        e.preventDefault();
        switchPage('domains');
        closeMobileDropdown();
    });
    document.getElementById('nav-settings-mobile')?.addEventListener('click', (e) => {
        e.preventDefault();
        switchPage('settings');
        closeMobileDropdown();
    });
    
    // 点击页面其他地方关闭dropdown
    document.addEventListener('click', (e) => {
        const dropdown = document.getElementById('mobile-dropdown');
        const menuBtn = document.getElementById('mobile-menu-btn');
        if (dropdown && menuBtn && !dropdown.contains(e.target) && !menuBtn.contains(e.target)) {
            closeMobileDropdown();
        }
    });
    
    // 监控控制
    elements.startMonitorBtn?.addEventListener('click', startMonitor);
    elements.stopMonitorBtn?.addEventListener('click', stopMonitor);
    elements.reloadBtn?.addEventListener('click', reloadDomains);
    elements.testNotificationBtn?.addEventListener('click', testNotification);
    
    // 搜索
    elements.searchBtn?.addEventListener('click', performSearch);
    elements.searchInput?.addEventListener('keypress', function(e) {
        if (e.key === 'Enter') {
            performSearch();
        }
    });
    elements.statusFilter?.addEventListener('change', performSearch);
    
    // 批量操作
    elements.selectAllCheckbox?.addEventListener('change', toggleSelectAll);
    elements.checkSelectedBtn?.addEventListener('click', checkSelectedDomains);
    elements.deleteSelectedBtn?.addEventListener('click', deleteSelectedDomains);
    
    // 域名管理
    elements.addSingleDomainBtn?.addEventListener('click', addSingleDomain);
    elements.addBatchDomainsBtn?.addEventListener('click', addBatchDomains);
    
    // 其他
    elements.refreshBtn?.addEventListener('click', refreshFromDatabase);
    elements.logoutBtn?.addEventListener('click', logout);
    
    // 个人设置页面事件（现在在设置页面中）
    document.getElementById('submit-change-password')?.addEventListener('click', handleChangePassword);
    document.getElementById('update-username')?.addEventListener('click', handleUpdateUsername);
    
    // 设置页面事件
    document.getElementById('saveSystemSettingsBtn')?.addEventListener('click', saveMonitorSettings);
    document.getElementById('saveSmtpBtn')?.addEventListener('click', saveSmtpSettings);
    document.getElementById('saveTelegramBtn')?.addEventListener('click', saveTelegramSettings);
    document.getElementById('testEmailBtn')?.addEventListener('click', testEmailSettings);
    document.getElementById('testTelegramBtn')?.addEventListener('click', testTelegramSettings);
    
    // 分页事件
    bindPaginationEvents();
    
    // 初始化分页选择器
    initializePaginationSelectors();
}

// 初始化导航
function initializeNavigation() {
    updateNavigation('dashboard');
}

// 页面切换
function switchPage(page) {
    // 隐藏所有页面
    document.querySelectorAll('.page-content').forEach(p => p.classList.add('hidden'));
    
    // 显示目标页面
    const targetPage = document.getElementById(`${page}-page`);
    if (targetPage) {
        targetPage.classList.remove('hidden');
    }
    
    // 更新导航状态 (只有主要页面才更新导航状态)
    if (['dashboard', 'domains', 'settings'].includes(page)) {
        updateNavigation(page);
    }
    currentPage = page;
    
    // 加载页面特定数据
    switch(page) {
        case 'dashboard':
            loadData();
            break;
        case 'domains':
            // 重置域名管理页面到第一页
            manageCurrentPage = 1;
            loadDomainManagement();
            break;
        case 'settings':
            loadSettings();
            break;
    }
}

// 更新导航状态
function updateNavigation(activePage) {
    document.querySelectorAll('.navbar .menu a').forEach(link => {
        link.classList.remove('active');
    });
    
    const activeLink = document.getElementById(`nav-${activePage}`);
    if (activeLink) {
        activeLink.classList.add('active');
    }
}

// 加载数据
async function loadData() {
    showLoading(true);
    
    try {
        // 构建查询参数
        const searchTerm = elements.searchInput?.value.trim() || '';
        const statusFilter = elements.statusFilter?.value || '';
        
        let apiUrl = `/api/domains?page=${dashboardCurrentPage}&limit=${itemsPerPage}`;
        if (searchTerm) {
            apiUrl += `&search=${encodeURIComponent(searchTerm)}`;
        }
        if (statusFilter) {
            apiUrl += `&status=${encodeURIComponent(statusFilter)}`;
        }
        
        const [domainsResponse, statsResponse] = await Promise.all([
            fetch(apiUrl),
            fetch('/api/stats')
        ]);
        
        if (!domainsResponse.ok || !statsResponse.ok) {
            throw new Error('加载数据失败');
        }
        
        const domainsData = await domainsResponse.json();
        const statsData = await statsResponse.json();
        
        domains = domainsData.domains || [];
        totalDomains = domainsData.total || 0; // 全量
        const totalFiltered = domainsData.total_filtered !== undefined ? domainsData.total_filtered : domains.length;
        window.__lastTotalFiltered = totalFiltered;
        
        console.log('[API响应] 当前页:', dashboardCurrentPage, '返回域名数:', domains.length, '总数:', totalDomains, '过滤后总数:', totalFiltered);
        
        // 同时更新全局变量，确保分页计算正确
        filteredDomains = domains;
        
        updateStatistics(statsData.monitor.status_counts);
        updateMonitorStatus(statsData.monitor.is_running);
        applyFilters();
        
    } catch (error) {
        console.error('加载数据失败:', error);
        showNotification('加载数据失败: ' + error.message, 'error');
    } finally {
        showLoading(false);
    }
}

// 更新统计信息
function updateStatistics(statusCounts) {
    const stats = {
        total: totalDomains, // 使用总域名数，而不是当前页域名数
        available: 0,
        grace: 0,
        pendingDelete: 0,
        redemption: 0
    };
    
    // 如果有状态统计数据，使用服务器端的完整统计
    if (statusCounts) {
        stats.available = statusCounts.available || 0;
        stats.grace = statusCounts.grace || 0;
        stats.pendingDelete = statusCounts.pending_delete || 0;
        stats.redemption = statusCounts.redemption || 0;
    } else {
        // 否则只统计当前页面的域名（作为降级方案）
        domains.forEach(domain => {
            switch (domain.status) {
                case 'available':
                    stats.available++;
                    break;
                case 'grace':
                    stats.grace++;
                    break;
                case 'pending_delete':
                    stats.pendingDelete++;
                    break;
                case 'redemption':
                    stats.redemption++;
                    break;
            }
        });
    }
    
    // 动画更新数字
        animateNumber(elements.totalDomains, totalDomains);
    animateNumber(elements.availableDomains, stats.available);
    animateNumber(elements.graceDomains, stats.grace);
    animateNumber(elements.pendingDeleteDomains, stats.pendingDelete);
    animateNumber(elements.redemptionDomains, stats.redemption);
}

// 数字动画
function animateNumber(element, targetValue) {
    if (!element) return;
    
    const currentValue = parseInt(element.textContent) || 0;
    const increment = targetValue > currentValue ? 1 : -1;
    const step = Math.abs(targetValue - currentValue) / 10;
    
    element.classList.add('updating');
    
    let current = currentValue;
    const timer = setInterval(() => {
        current += increment * Math.max(1, Math.floor(step));
        
        if ((increment > 0 && current >= targetValue) || (increment < 0 && current <= targetValue)) {
            current = targetValue;
            clearInterval(timer);
            element.classList.remove('updating');
        }
        
        element.textContent = current;
    }, 50);
}

// 更新监控状态
function updateMonitorStatus(isRunning) {
    const status = elements.monitorStatus;
    if (!status) return;
    
    status.className = 'badge badge-lg';
    
    if (isRunning) {
        status.textContent = '运行中';
        status.classList.add('badge-success');
        elements.startMonitorBtn.disabled = true;
        elements.stopMonitorBtn.disabled = false;
    } else {
        status.textContent = '已停止';
        status.classList.add('badge-error');
        elements.startMonitorBtn.disabled = false;
        elements.stopMonitorBtn.disabled = true;
    }
}

// 执行搜索（重置到第一页）
function performSearch() {
    dashboardCurrentPage = 1; // 搜索时重置到第一页
    loadData();
}

// 应用过滤器
function applyFilters() {
    // 直接显示当前页面的数据
    displayDomainsWithPagination();
}

// 渲染域名表格
function renderDomainTable() {
    const tbody = elements.domainTableBody;
    if (!tbody) return;
    
    tbody.innerHTML = '';
    
    if (filteredDomains.length === 0) {
        elements.emptyState?.classList.remove('hidden');
        return;
    }
    
    elements.emptyState?.classList.add('hidden');
    
    filteredDomains.forEach(domain => {
        const row = createDomainRow(domain);
        tbody.appendChild(row);
    });
}

// 创建域名行
function createDomainRow(domain) {
    const row = document.createElement('tr');
    row.className = 'hover';
    
    // Determine effective status
    let effectiveStatus = domain.status || 'unknown';
    // Check if it's effectively 'checking'
    if (effectiveStatus === 'unknown' && (domain.query_method === 'checking' || domain.query_method === 'pending')) {
        effectiveStatus = 'checking';
    }

    const statusClass = getStatusColor(effectiveStatus);
    const statusText = getStatusText(effectiveStatus);
    const lastChecked = formatDateTime(domain.last_checked);
    const expiryDate = domain.expiry_date ? formatDateTime(domain.expiry_date) : formatFieldValue('', 'expiry_date', domain);
    
    // 如果有错误信息，添加提示
    const errorTooltip = domain.error_message ? `title="${domain.error_message}"` : '';
    
    row.innerHTML = `
        <td>
            <input type="checkbox" class="checkbox" name="domainCheckbox" value="${domain.name}">
        </td>
        <td>
            <a href="#" class="domain-link" data-domain="${domain.name}">${domain.name}</a>
        </td>
        <td>
            <div class="badge ${statusClass}" ${errorTooltip}>${statusText}</div>
            ${domain.error_message ? '<span class="text-xs text-error ml-1" title="' + domain.error_message + '">!</span>' : ''}
        </td>
        <td>${formatFieldValue(domain.registrar, 'registrar', domain)}</td>
        <td>${expiryDate}</td>
        <td>
            <div class="text-sm">${lastChecked}</div>
        </td>
        <td>
            <div class="join">
                <button class="btn btn-sm btn-primary join-item" onclick="checkDomain('${domain.name}')">
                    检查
                </button>
                <button class="btn btn-sm btn-ghost join-item" onclick="viewDomainDetails('${domain.name}')">
                    详情
                </button>
            </div>
        </td>
    `;
    
    // 添加域名链接点击事件
    const domainLink = row.querySelector('.domain-link');
    domainLink?.addEventListener('click', function(e) {
        e.preventDefault();
        viewDomainDetails(domain.name);
    });
    
    return row;
}

// 获取状态文本
function getStatusText(status) {
    const statusMap = {
        'available': '可注册',
        'registered': '已注册',
        'grace': '宽限期',
        'redemption': '赎回期',
        'pending_delete': '待删除',
        'error': '查询错误',
        'checking': '正在查询',
        'unknown': '未知状态'
    };
    
    return statusMap[status] || '未知状态';
}

// 格式化字段值，处理不支持的字段
function formatFieldValue(value, fieldType, domain) {
    // 如果是查询错误状态，显示"查询错误"
    if (domain && domain.status === 'error') {
        if (fieldType === 'registrar' || fieldType === 'created_date' || 
            fieldType === 'expiry_date' || fieldType === 'updated_date') {
            return '<span class="text-error">查询错误</span>';
        }
    }
    
    // 如果是正在查询状态，显示"正在查询"
    const isChecking = (domain && domain.status === 'checking') || 
                       (domain && domain.status === 'unknown' && (domain.query_method === 'checking' || domain.query_method === 'pending'));

    if (isChecking) {
        if (fieldType === 'registrar' || fieldType === 'created_date' || 
            fieldType === 'expiry_date' || fieldType === 'updated_date') {
            return '<span class="text-info">正在查询...</span>';
        }
    }

    // 如果是未知状态，显示"未知"
    if (domain && domain.status === 'unknown') {
        if (fieldType === 'registrar' || fieldType === 'created_date' || 
            fieldType === 'expiry_date' || fieldType === 'updated_date') {
            return '<span class="text-base-content/60">未知</span>';
        }
    }
    
    if (!value || value === '-') {
        // 如果字段为空且域名状态不是available、error、checking、unknown，显示不支持提示
        if (domain && domain.status !== 'available' && domain.status !== 'error' && domain.status !== 'checking' && domain.status !== 'unknown') {
            const tld = domain.name.split('.').pop();
            switch (fieldType) {
                case 'registrar':
                    return `<span class="unsupported-field">${tld}后缀不支持注册商信息</span>`;
                case 'created_date':
                    return `<span class="unsupported-field">${tld}后缀不支持创建时间</span>`;
                case 'expiry_date':
                    return `<span class="unsupported-field">${tld}后缀不支持过期时间</span>`;
                case 'updated_date':
                    return `<span class="unsupported-field">${tld}后缀不支持更新时间</span>`;
            }
        }
        return '-';
    }
    
    // 检查是否为不支持的字段提示
    if (typeof value === 'string' && value.includes('该后缀不支持')) {
        return `<span class="unsupported-field">${value}</span>`;
    }
    
    return value;
}

// 获取状态颜色类
function getStatusColor(status) {
    const colorMap = {
        'available': 'badge-success',
        'registered': 'badge-primary', 
        'grace': 'badge-info',
        'redemption': 'badge-warning',
        'pending_delete': 'badge-error',
        'error': 'badge-error',
        'checking': 'badge-info',
        'unknown': 'badge-ghost'
    };
    return colorMap[status] || 'badge-ghost';
}

// 显示加载状态
function showLoading(show) {
    const indicator = elements.loadingIndicator;
    if (indicator) {
        indicator.style.display = show ? 'flex' : 'none';
    }
}

// 显示通知
function showNotification(message, type = 'info', duration = 5000) {
    const toast = document.createElement('div');
    toast.className = `alert alert-${type} shadow-lg`;
    
    toast.innerHTML = `
        <div>
            <span>${message}</span>
        </div>
    `;
    
    elements.notifications?.appendChild(toast);
    
    // 自动移除
    setTimeout(() => {
        toast?.remove();
    }, duration);
}

// 监控控制功能
async function startMonitor() {
    try {
        const response = await fetch('/api/monitor/start', { method: 'POST' });
        if (!response.ok) throw new Error('启动监控失败');
        
        showNotification('监控已启动', 'success');
        updateMonitorStatus(true);
    } catch (error) {
        showNotification('启动监控失败: ' + error.message, 'error');
    }
}

async function stopMonitor() {
    try {
        const response = await fetch('/api/monitor/stop', { method: 'POST' });
        if (!response.ok) throw new Error('停止监控失败');
        
        showNotification('监控已停止', 'success');
        updateMonitorStatus(false);
    } catch (error) {
        showNotification('停止监控失败: ' + error.message, 'error');
    }
}

async function reloadDomains() {
    try {
        const response = await fetch('/api/monitor/reload', { method: 'POST' });
        if (!response.ok) throw new Error('重新加载失败');
        
        showNotification('域名列表已重新加载', 'success');
        loadData();
    } catch (error) {
        showNotification('重新加载失败: ' + error.message, 'error');
    }
}

async function testNotification() {
    try {
        const response = await fetch('/api/notification/test', { method: 'POST' });
        const result = await response.json();
        
        // 根据返回的状态显示不同的消息
        if (result.status === 'warning') {
            showNotification(result.message, 'warning');
        } else if (result.status === 'error') {
            showNotification(result.message, 'error');
        } else {
            // 构建详细的状态消息
            let message = '';
            if (result.email_status && result.telegram_status) {
                message = `邮箱: ${result.email_status}, Telegram: ${result.telegram_status}`;
            } else {
                message = result.message;
            }
            
            // 根据整体状态显示不同类型的通知
            if (result.status === 'success') {
                showNotification(message, 'success');
            } else {
                showNotification(message, 'warning');
            }
        }
    } catch (error) {
        showNotification('测试通知失败: ' + error.message, 'error');
    }
}

// 域名操作
async function checkDomain(domainName) {
    try {
        showNotification(`正在检查域名 ${domainName}...`, 'info');
        
        const response = await fetch(`/api/domain/check/${domainName}`, { method: 'POST' });
        if (!response.ok) throw new Error('检查域名失败');
        
        const result = await response.json();
        showNotification(`域名 ${domainName} 检查完成`, 'success');
        
        // 更新表格中的数据
        const index = domains.findIndex(d => d.name === domainName);
        if (index !== -1) {
            domains[index] = result;
            updateStatistics();
            applyFilters();
        }
    } catch (error) {
        showNotification(`检查域名失败: ${error.message}`, 'error');
    }
}

async function viewDomainDetails(domainName) {
    try {
        const response = await fetch(`/api/domain/${domainName}`);
        if (!response.ok) throw new Error('获取域名详情失败');
        
        const domain = await response.json();
        showDomainModal(domain);
    } catch (error) {
        showNotification('获取域名详情失败: ' + error.message, 'error');
    }
}

// 显示域名详情模态框
function showDomainModal(domain) {
    const modal = document.getElementById('domainModal');
    const title = document.getElementById('domainModalTitle');
    const details = document.getElementById('domainDetails');
    
    if (!modal || !title || !details) return;
    
    title.textContent = `${domain.name} - 域名详情`;
    
    const lastChecked = formatDateTime(domain.last_checked);
    const createdDate = domain.created_date ? formatDateTime(domain.created_date) : formatFieldValue('', 'created_date', domain);
    const expiryDate = domain.expiry_date ? formatDateTime(domain.expiry_date) : formatFieldValue('', 'expiry_date', domain);
    const updatedDate = domain.updated_date ? formatDateTime(domain.updated_date) : formatFieldValue('', 'updated_date', domain);
    
    details.innerHTML = `
        <div class="domain-detail-grid">
            <div class="domain-detail-item">
                <div class="domain-detail-label">域名</div>
                <div class="domain-detail-value">${domain.name}</div>
            </div>
            <div class="domain-detail-item">
                <div class="domain-detail-label">状态</div>
                <div class="domain-detail-value">
                    <span class="badge status-${domain.status}">${getStatusText(domain.status)}</span>
                </div>
            </div>
            <div class="domain-detail-item">
                <div class="domain-detail-label">注册商</div>
                <div class="domain-detail-value">${formatFieldValue(domain.registrar, 'registrar', domain)}</div>
            </div>
            <div class="domain-detail-item">
                <div class="domain-detail-label">创建时间</div>
                <div class="domain-detail-value">${createdDate}</div>
            </div>
            <div class="domain-detail-item">
                <div class="domain-detail-label">过期时间</div>
                <div class="domain-detail-value">${expiryDate}</div>
            </div>
            <div class="domain-detail-item">
                <div class="domain-detail-label">更新时间</div>
                <div class="domain-detail-value">${updatedDate}</div>
            </div>
            <div class="domain-detail-item">
                <div class="domain-detail-label">查询方式</div>
                <div class="domain-detail-value">${domain.query_method || '-'}</div>
            </div>
            <div class="domain-detail-item">
                <div class="domain-detail-label">最后检查</div>
                <div class="domain-detail-value">${lastChecked}</div>
            </div>
            ${domain.name_servers && domain.name_servers.length > 0 ? `
            <div class="domain-detail-item col-span-full">
                <div class="domain-detail-label">名称服务器</div>
                <div class="domain-detail-value">
                    <ul class="list-disc list-inside space-y-1">
                        ${domain.name_servers.map(ns => `<li>${ns}</li>`).join('')}
                    </ul>
                </div>
            </div>
            ` : ''}
            ${domain.error_message ? `
            <div class="domain-detail-item col-span-full">
                <div class="domain-detail-label">查询错误信息</div>
                <div class="domain-detail-value">
                    <div class="alert alert-error">
                        <svg xmlns="http://www.w3.org/2000/svg" class="stroke-current shrink-0 h-6 w-6" fill="none" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2m7-2a9 9 0 11-18 0 9 9 0 0118 0z" />
                        </svg>
                        <span class="text-sm">${domain.error_message}</span>
                    </div>
                </div>
            </div>
            ` : ''}
            ${domain.status !== 'error' && domain.status !== 'unknown1' ? `
            <div class="domain-detail-item col-span-full">
                <div class="domain-detail-label">WHOIS元数据</div>
                <div class="domain-detail-value">
                    <button class="btn btn-sm btn-primary" onclick="viewWhoisRaw('${domain.name}', ${domain.whois_raw ? 'true' : 'false'})">
                        ${domain.whois_raw ? '查看WHOIS原始数据' : '查询WHOIS原始数据'}
                    </button>
                </div>
            </div>
            ` : ''}
        </div>
    `;
    
    modal.showModal();
}

// 查看WHOIS原始数据（优先从数据库获取，没有才查询）
async function viewWhoisRaw(domainName, hasData) {
    try {
        // 如果数据库中有数据，直接显示
        if (hasData) {
            const response = await fetch(`/api/domain/${domainName}`);
            if (!response.ok) throw new Error('获取域名详情失败');
            
            const domain = await response.json();
            if (domain.whois_raw) {
                showWhoisRawData(domainName, domain.whois_raw);
                return;
            }
        }
        
        // 数据库中没有数据，查询并保存
        showNotification(`正在查询 ${domainName} 的WHOIS原始数据...`, 'info');
        
        const response = await fetch(`/api/domain/whois-raw/${domainName}`, { method: 'POST' });
        if (!response.ok) {
            const result = await response.json();
            throw new Error(result.error || '查询WHOIS原始数据失败');
        }
        
        const result = await response.json();
        showNotification(`WHOIS原始数据查询成功`, 'success');
        
        // 显示WHOIS原始数据
        showWhoisRawData(domainName, result.whois_raw);
    } catch (error) {
        showNotification(`查询WHOIS原始数据失败: ${error.message}`, 'error');
    }
}

// 显示WHOIS原始数据模态框
function showWhoisRawData(domainName, whoisRaw) {
    const modal = document.getElementById('domainModal');
    const title = document.getElementById('domainModalTitle');
    const details = document.getElementById('domainDetails');
    
    if (!modal || !title || !details) return;
    
    title.textContent = `${domainName} - WHOIS原始数据`;
    
    details.innerHTML = `
        <div class="w-full">
            <div class="mb-4 flex justify-between items-center">
                <span class="text-sm text-base-content/60">共 ${whoisRaw.length} 字符</span>
                <button class="btn btn-sm btn-secondary" onclick="copyWhoisRaw()">
                    复制到剪贴板
                </button>
            </div>
            <textarea id="whoisRawContent" class="textarea textarea-bordered w-full h-96 font-mono text-xs" readonly>${whoisRaw}</textarea>
        </div>
    `;
    
    modal.showModal();
}

// 复制WHOIS原始数据到剪贴板
function copyWhoisRaw() {
    const textarea = document.getElementById('whoisRawContent');
    if (textarea) {
        textarea.select();
        document.execCommand('copy');
        showNotification('已复制到剪贴板', 'success');
    }
}

function formatDateTime(value) {
    if (!value) return '-';
    const date = new Date(value);
    if (isNaN(date.getTime())) return '-';
    return date.toLocaleString('zh-CN', dateTimeOptions);
}

// 批量操作
function toggleSelectAll(event) {
    const checkboxes = document.querySelectorAll('input[name="domainCheckbox"]');
    const isChecked = event.target.checked;
    
    checkboxes.forEach(checkbox => {
        checkbox.checked = isChecked;
    });
    
    // 同步全选框状态
    if (elements.selectAllCheckbox) elements.selectAllCheckbox.checked = isChecked;
}

async function checkSelectedDomains() {
    const selectedDomains = Array.from(document.querySelectorAll('input[name="domainCheckbox"]:checked'))
        .map(checkbox => checkbox.value);
    
    if (selectedDomains.length === 0) {
        showNotification('请选择要检查的域名', 'warning');
        return;
    }
    
    showNotification(`正在检查 ${selectedDomains.length} 个域名...`, 'info');
    
    // 并行检查所有选中的域名
    const promises = selectedDomains.map(domain => checkDomain(domain));
    
    try {
        await Promise.all(promises);
        showNotification('所有选中域名检查完成', 'success');
    } catch (error) {
        showNotification('部分域名检查失败', 'warning');
    }
}

// 删除选中的域名
async function deleteSelectedDomains() {
    const selectedDomains = Array.from(document.querySelectorAll('input[name="domainCheckbox"]:checked'))
        .map(checkbox => checkbox.value);
    
    if (selectedDomains.length === 0) {
        showNotification('请选择要删除的域名', 'warning');
        return;
    }
    
    if (!confirm(`确定要删除这 ${selectedDomains.length} 个域名吗？`)) {
        return;
    }
    
    showNotification(`正在删除 ${selectedDomains.length} 个域名...`, 'info');
    
    let successCount = 0;
    let errorCount = 0;
    
    for (const domain of selectedDomains) {
        try {
            const response = await fetch(`/api/domain/remove/${domain}`, {
                method: 'DELETE'
            });
            
            if (!response.ok) {
                const result = await response.json();
                throw new Error(result.error || `删除域名 ${domain} 失败`);
            }
            successCount++;
        } catch (error) {
            console.error(`删除域名 ${domain} 失败:`, error);
            errorCount++;
        }
    }
    
    if (errorCount === 0) {
        showNotification(`成功删除 ${successCount} 个域名`, 'success');
    } else {
        showNotification(`删除完成：成功 ${successCount} 个，失败 ${errorCount} 个`, 'warning');
    }
    
    // 清空全选框
    if (elements.selectAllCheckbox) elements.selectAllCheckbox.checked = false;
    
    // 重新加载数据
    await loadData();
    if (currentPage === 'domains') {
        await loadDomainManagement();
    }
}

// 域名管理功能
async function loadDomainManagement() {
    try {
        const response = await fetch(`/api/domains?page=${manageCurrentPage}&limit=${itemsPerPage}`);
        if (!response.ok) throw new Error('Failed to fetch domains');
        
        const data = await response.json();
        domains = data.domains || [];
        totalManageDomains = data.total || 0;
        
        displayManageDomainsWithPagination();
    } catch (error) {
        console.error('Error loading domain management data:', error);
        showNotification('加载域名管理数据失败: ' + error.message, 'error');
    }
}

function renderManageDomainTable() {
    const tbody = elements.manageDomainTableBody;
    if (!tbody) return;
    
    tbody.innerHTML = '';
    
    domains.forEach(domain => {
        const row = document.createElement('tr');
        row.className = 'domain-item hover'; // 添加CSS类名
        row.innerHTML = `
            <td>${domain.name}</td>
            <td>-</td>
            <td>
                <span class="badge status-${domain.status}">${getStatusText(domain.status)}</span>
            </td>
            <td>
                <button class="btn btn-sm btn-error" onclick="removeDomain('${domain.name}')">
                    删除
                </button>
            </td>
        `;
        tbody.appendChild(row);
    });
}

async function addSingleDomain() {
    const input = elements.singleDomainInput;
    if (!input) return;
    
    const domain = input.value.trim();
    if (!domain) {
        showNotification('请输入域名', 'warning');
        return;
    }
    
    try {
        const response = await fetch('/api/domain/add', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({ domain: domain })
        });
        
        const result = await response.json();
        
        // 检查返回的status字段
        if (result.status === 'error') {
            // 如果是不支持的后缀，显示模态框
            if (result.message && result.message.includes('不支持进行监控')) {
                showUnsupportedDomainsModal([domain]);
            } else {
                showNotification(result.message || '添加域名失败', 'error');
            }
            return;
        }
        
        if (!response.ok) {
            throw new Error(result.error || result.message || '添加域名失败');
        }
        
        showNotification(`域名 ${domain} 添加成功`, 'success');
        input.value = '';
        loadData();
        if (currentPage === 'domains') {
            loadDomainManagement();
        }
    } catch (error) {
        showNotification('添加域名失败: ' + error.message, 'error');
    }
}

async function addBatchDomains() {
    const textarea = elements.batchDomainInput;
    if (!textarea) return;
    
    const domainsText = textarea.value.trim();
    if (!domainsText) {
        showNotification('请输入域名列表', 'warning');
        return;
    }
    
    const domainList = domainsText.split('\n')
        .map(d => d.trim())
        .filter(d => d);
    
    if (domainList.length === 0) {
        showNotification('没有有效的域名', 'warning');
        return;
    }
    
    try {
        const response = await fetch('/api/domain/batch-add', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({ domains: domainList })
        });
        
        const result = await response.json();
        
        if (!response.ok) {
            throw new Error(result.error || '批量添加失败');
        }
        
        let message = `成功添加 ${result.added_count} 个域名`;
        if (result.invalid_count > 0) {
            message += `，${result.invalid_count} 个域名无效`;
        }
        if (result.unsupported_count > 0) {
            message += `，${result.unsupported_count} 个域名后缀不支持`;
        }
        
        showNotification(message, (result.invalid_count > 0 || result.unsupported_count > 0) ? 'warning' : 'success');
        
        // 如果有不支持的域名，显示模态框
        if (result.unsupported_count > 0 && result.unsupported_domains) {
            showUnsupportedDomainsModal(result.unsupported_domains);
        }
        
        textarea.value = '';
        loadData();
        if (currentPage === 'domains') {
            loadDomainManagement();
        }
    } catch (error) {
        showNotification('批量添加失败: ' + error.message, 'error');
    }
}

// 显示不支持的域名模态框
function showUnsupportedDomainsModal(domains) {
    const modal = document.createElement('div');
    modal.className = 'modal modal-open';
    modal.innerHTML = `
        <div class="modal-box">
            <h3 class="font-bold text-lg mb-4">以下域名后缀不支持监控</h3>
            <div class="max-h-96 overflow-y-auto">
                <ul class="list-disc list-inside space-y-1">
                    ${domains.map(d => `<li class="text-error">${d}</li>`).join('')}
                </ul>
            </div>
            <div class="modal-action">
                <button class="btn btn-primary" onclick="this.closest('.modal').remove()">确定</button>
            </div>
        </div>
    `;
    document.body.appendChild(modal);
}

// 设置页面
function loadSettings() {
    // 加载系统设置和配置
    loadSystemSettings();
    loadConfigSettings();
}

async function loadSystemSettings() {
    try {
        const response = await fetch('/api/stats');
        if (!response.ok) throw new Error('加载设置失败');
        
        const stats = await response.json();
        
        // 显示系统统计信息
        const systemStats = document.getElementById('systemStats');
        if (systemStats) {
            systemStats.innerHTML = `
                <div class="stat">
                    <div class="stat-title">监控状态</div>
                    <div class="stat-value">${stats.monitor.is_running ? '运行中' : '已停止'}</div>
                </div>
                <div class="stat">
                    <div class="stat-title">域名数量</div>
                    <div class="stat-value">${stats.monitor.domain_count}</div>
                </div>
                <div class="stat">
                    <div class="stat-title">Worker数量</div>
                    <div class="stat-value">${stats.monitor.worker_count}</div>
                </div>
                <div class="stat">
                    <div class="stat-title">活跃会话</div>
                    <div class="stat-value">${stats.auth.active_sessions || 0}</div>
                </div>
            `;
        }
        
    } catch (error) {
        console.error('加载设置失败:', error);
        showNotification('加载设置失败: ' + error.message, 'error');
    }
}

// 加载配置设置
async function loadConfigSettings() {
    try {
        const response = await fetch('/api/settings');
        if (!response.ok) throw new Error('加载配置失败');
        
        const settings = await response.json();
        
        // 填充监控设置
        if (settings.monitor) {
            const checkIntervalInput = document.getElementById('checkIntervalInput');
            const concurrentLimitInput = document.getElementById('concurrentLimitInput');
            const timeoutInput = document.getElementById('timeoutInput');

            if (checkIntervalInput) {
                checkIntervalInput.value = settings.monitor.check_interval;
            }
            if (concurrentLimitInput) {
                concurrentLimitInput.value = settings.monitor.concurrent_limit;
            }
            if (timeoutInput) {
                timeoutInput.value = settings.monitor.timeout;
            }
        }
        
        // 填充SMTP设置
        if (settings.smtp) {
            document.getElementById('smtpHost').value = settings.smtp.host || '';
            document.getElementById('smtpPort').value = settings.smtp.port || 587;
            document.getElementById('smtpUser').value = settings.smtp.user || '';
            document.getElementById('smtpPass').value = settings.smtp.password || ''; // 填充密码
            document.getElementById('smtpFrom').value = settings.smtp.from || '';
            document.getElementById('smtpTo').value = settings.smtp.to || '';
            document.getElementById('emailNotificationToggle').checked = settings.smtp.enabled || false;
        }
        
        // 填充Telegram设置
        if (settings.telegram) {
            document.getElementById('telegramBotToken').value = settings.telegram.bot_token || ''; // 填充bot token
            document.getElementById('telegramChatId').value = settings.telegram.chat_id || '';
            document.getElementById('telegramNotificationToggle').checked = settings.telegram.enabled || false;
        }
        
        // 填充用户名
        if (settings.username) {
            document.getElementById('profile-username').value = settings.username || '';
        }
        
    } catch (error) {
        console.error('加载配置失败:', error);
        showNotification('加载配置失败: ' + error.message, 'error');
    }
}

// 删除域名
async function removeDomain(domainName) {
    if (!confirm(`确定要删除域名 ${domainName} 吗？`)) {
        return;
    }
    
    try {
        const response = await fetch(`/api/domain/remove/${domainName}`, {
            method: 'DELETE'
        });
        
        const result = await response.json();
        
        if (!response.ok) {
            throw new Error(result.error || '删除域名失败');
        }
    
    showNotification(`域名 ${domainName} 删除成功`, 'success');
    
    // 立即重新加载数据
    await loadData();
    if (currentPage === 'domains') {
        await loadDomainManagement();
    }
    } catch (error) {
        showNotification('删除域名失败: ' + error.message, 'error');
    }
}

// 刷新数据（从数据库获取，不重新查询）
async function refreshFromDatabase() {
    showLoading(true);
    
    try {
        const searchTerm = elements.searchInput?.value.trim() || '';
        const statusFilter = elements.statusFilter?.value || '';
        
        let apiUrl = `/api/domains?page=${dashboardCurrentPage}&limit=${itemsPerPage}`;
        if (searchTerm) {
            apiUrl += `&search=${encodeURIComponent(searchTerm)}`;
        }
        if (statusFilter) {
            apiUrl += `&status=${encodeURIComponent(statusFilter)}`;
        }
        
        const [domainsResponse, statsResponse] = await Promise.all([
            fetch(apiUrl),
            fetch('/api/stats')
        ]);
        
        if (!domainsResponse.ok || !statsResponse.ok) {
            throw new Error('加载数据失败');
        }
        
        const domainsData = await domainsResponse.json();
        const statsData = await statsResponse.json();
        
        domains = domainsData.domains || [];
        totalDomains = domainsData.total || 0;
        const totalFiltered = domainsData.total_filtered !== undefined ? domainsData.total_filtered : domains.length;
        window.__lastTotalFiltered = totalFiltered;
        
        filteredDomains = domains;
        
        updateStatistics(statsData.monitor.status_counts);
        updateMonitorStatus(statsData.monitor.is_running);
        applyFilters();
        
        showNotification('数据已从数据库刷新', 'success');
    } catch (error) {
        console.error('刷新数据失败:', error);
        showNotification('刷新数据失败: ' + error.message, 'error');
    } finally {
        showLoading(false);
    }
}

// 登出
function logout() {
    if (confirm('确定要退出登录吗？')) {
        window.location.href = '/logout';
    }
}

// 绑定分页事件
function bindPaginationEvents() {
    // 仪表板分页
    document.getElementById('prevPageBtn')?.addEventListener('click', () => {
        if (dashboardCurrentPage > 1) {
            dashboardCurrentPage--;
            loadData();
        }
    });
    
    document.getElementById('nextPageBtn')?.addEventListener('click', () => {
        const totalPages = Math.ceil(totalDomains / itemsPerPage);
        if (dashboardCurrentPage < totalPages) {
            dashboardCurrentPage++;
            loadData();
        }
    });
    
    // 仪表板分页大小选择
    document.getElementById('pageSizeSelect')?.addEventListener('change', (e) => {
        const newSize = parseInt(e.target.value);
        if (newSize >= 20 && newSize <= 100) {
            itemsPerPage = newSize;
            dashboardCurrentPage = 1; // 重置到第一页
            loadData();
            
            // 同步域名管理的分页大小选择器
            const manageSelect = document.getElementById('managePageSizeSelect');
            if (manageSelect) {
                manageSelect.value = newSize.toString();
            }
        }
    });
    
    // 域名管理分页
    document.getElementById('managePrevPageBtn')?.addEventListener('click', () => {
        if (manageCurrentPage > 1) {
            manageCurrentPage--;
            loadDomainManagement();
        }
    });
    
    document.getElementById('manageNextPageBtn')?.addEventListener('click', () => {
        const totalPages = Math.ceil(totalManageDomains / itemsPerPage);
        if (manageCurrentPage < totalPages) {
            manageCurrentPage++;
            loadDomainManagement();
        }
    });
    
    // 域名管理分页大小选择
    document.getElementById('managePageSizeSelect')?.addEventListener('change', (e) => {
        const newSize = parseInt(e.target.value);
        if (newSize >= 20 && newSize <= 100) {
            itemsPerPage = newSize;
            manageCurrentPage = 1; // 重置到第一页
            loadDomainManagement();
            
            // 同步仪表板的分页大小选择器
            const dashboardSelect = document.getElementById('pageSizeSelect');
            if (dashboardSelect) {
                dashboardSelect.value = newSize.toString();
            }
        }
    });
}

// 初始化分页选择器
function initializePaginationSelectors() {
    // 设置默认值为20
    const dashboardSelect = document.getElementById('pageSizeSelect');
    const manageSelect = document.getElementById('managePageSizeSelect');
    
    if (dashboardSelect) {
        dashboardSelect.value = itemsPerPage.toString();
    }
    
    if (manageSelect) {
        manageSelect.value = itemsPerPage.toString();
    }
}

// 处理修改密码
async function handleChangePassword() {
    const currentPassword = document.getElementById('current-password').value;
    const newPassword = document.getElementById('new-password').value;
    const confirmPassword = document.getElementById('confirm-password').value;
    
    if (!currentPassword || !newPassword || !confirmPassword) {
        showNotification('请填写所有密码字段', 'error');
        return;
    }
    
    if (newPassword !== confirmPassword) {
        showNotification('新密码和确认密码不匹配', 'error');
        return;
    }
    
    if (newPassword.length < 6) {
        showNotification('新密码长度至少6位', 'error');
        return;
    }
    
    try {
        const response = await fetch('/api/change-password', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                current_password: currentPassword,
                new_password: newPassword
            })
        });
        
        if (response.ok) {
            showNotification('密码修改成功', 'success');
            // 清空密码输入框
            document.getElementById('current-password').value = '';
            document.getElementById('new-password').value = '';
            document.getElementById('confirm-password').value = '';
        } else {
            const result = await response.json();
            showNotification(result.error || '修改密码失败', 'error');
        }
    } catch (error) {
        showNotification('修改密码失败: ' + error.message, 'error');
    }
}

// 处理更新用户名
async function handleUpdateUsername() {
    const newUsername = document.getElementById('profile-username').value.trim();
    
    if (!newUsername) {
        showNotification('请输入用户名', 'error');
        return;
    }
    
    if (newUsername.length < 3) {
        showNotification('用户名长度至少3位', 'error');
        return;
    }
    
    try {
        const response = await fetch('/api/update-username', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                username: newUsername
            })
        });
        
        if (response.ok) {
            showNotification('用户名更新成功', 'success');
        } else {
            const result = await response.json();
            showNotification(result.error || '更新用户名失败', 'error');
        }
    } catch (error) {
        showNotification('更新用户名失败: ' + error.message, 'error');
    }
}

// 保存SMTP设置
async function saveSmtpSettings() {
    const smtpData = {
        host: document.getElementById('smtpHost').value.trim(),
        port: parseInt(document.getElementById('smtpPort').value) || 587,
        user: document.getElementById('smtpUser').value.trim(),
        password: document.getElementById('smtpPass').value.trim(),
        from: document.getElementById('smtpFrom').value.trim(),
        to: document.getElementById('smtpTo').value.trim(),
        enabled: document.getElementById('emailNotificationToggle').checked
    };
    
    if (smtpData.enabled && (!smtpData.host || !smtpData.user || !smtpData.from || !smtpData.to)) {
        showNotification('请填写完整的SMTP信息', 'error');
        return;
    }
    
    try {
        const response = await fetch('/api/settings/smtp', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify(smtpData)
        });
        
        if (response.ok) {
            showNotification('SMTP设置保存成功', 'success');
        } else {
            const result = await response.json();
            showNotification(result.error || '保存SMTP设置失败', 'error');
        }
    } catch (error) {
        showNotification('保存SMTP设置失败: ' + error.message, 'error');
    }
}

// 保存Telegram设置
async function saveTelegramSettings() {
    const telegramData = {
        bot_token: document.getElementById('telegramBotToken').value.trim(),
        chat_id: document.getElementById('telegramChatId').value.trim(),
        enabled: document.getElementById('telegramNotificationToggle').checked
    };
    
    if (telegramData.enabled && (!telegramData.bot_token || !telegramData.chat_id)) {
        showNotification('请填写完整的Telegram信息', 'error');
        return;
    }
    
    try {
        const response = await fetch('/api/settings/telegram', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify(telegramData)
        });
        
        if (response.ok) {
            showNotification('Telegram设置保存成功', 'success');
        } else {
            const result = await response.json();
            showNotification(result.error || '保存Telegram设置失败', 'error');
        }
    } catch (error) {
        showNotification('保存Telegram设置失败: ' + error.message, 'error');
    }
}

// 保存监控设置
async function saveMonitorSettings() {
    const checkIntervalInput = document.getElementById('checkIntervalInput');
    const concurrentLimitInput = document.getElementById('concurrentLimitInput');
    const timeoutInput = document.getElementById('timeoutInput');
    
    // 获取原始值
    const checkInterval = parseInt(checkIntervalInput.value);
    const concurrentLimit = parseInt(concurrentLimitInput.value);
    const timeout = parseInt(timeoutInput.value);
    
    // 验证参数
    if (!checkInterval || checkInterval < 5) {
        showNotification('检查间隔不能小于5秒', 'error');
        checkIntervalInput.value = 5; // 重置为最小值
        return;
    }
    
    if (checkInterval < 30) {
        // 建议提示
        showNotification('建议：检查间隔设置小于30秒可能会触发频率限制', 'warning', 5000);
    }
    
    if (!concurrentLimit || concurrentLimit < 1 || concurrentLimit > 1000) {
        showNotification('并发限制必须在1-1000之间', 'error');
        return;
    }
    
    if (!timeout || timeout < 1 || timeout > 120) {
        showNotification('超时时间必须在1-120秒之间', 'error');
        return;
    }
    
    try {
        const response = await fetch('/api/settings/monitor', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                check_interval: checkInterval, // 直接发送秒数
                concurrent_limit: concurrentLimit,
                timeout: timeout
            })
        });
        
        const result = await response.json();
        
        if (response.ok) {
            showNotification(result.message || '监控设置保存成功', 'success');
            
            // 重新加载页面数据，因为后端可能已经更新了配置
            setTimeout(loadSystemSettings, 500);
        } else {
            showNotification(result.error || result.message || '保存监控设置失败', 'error');
        }
    } catch (error) {
        showNotification('保存监控设置失败: ' + error.message, 'error');
    }
}

// 测试邮件设置
async function testEmailSettings() {
    try {
        showNotification('正在发送测试邮件...', 'info');
        const response = await fetch('/api/test/email', { method: 'POST' });
        const result = await response.json();
        
        // 判断是否成功
        if (result.status === 'success') {
            showNotification(result.message, 'success');
        } else {
            showNotification(result.message, 'error');
        }
    } catch (error) {
        showNotification('测试邮件发送失败: ' + error.message, 'error');
    }
}

// 测试Telegram设置
async function testTelegramSettings() {
    try {
        showNotification('正在发送测试Telegram消息...', 'info');
        const response = await fetch('/api/test/telegram', { method: 'POST' });
        const result = await response.json();
        
        // 判断是否成功
        if (result.status === 'success') {
            showNotification(result.message, 'success');
        } else {
            showNotification(result.message, 'error');
        }
    } catch (error) {
        showNotification('测试Telegram消息发送失败: ' + error.message, 'error');
    }
}



// 带分页的显示域名（服务器端分页）
function displayDomainsWithPagination() {
    const tableBody = elements.domainTableBody;
    if (!tableBody) return;
    
    // 确保隐藏加载动画
    showLoading(false);
    
    // 清空表格
    tableBody.innerHTML = '';
    
    // 显示域名
    const totalFiltered = domainsDataTotalFiltered(domains.length);

    if (domains.length === 0) {
        if (totalDomains === 0) {
            // 完全没有数据
            elements.emptyState?.classList.remove('hidden');
            document.getElementById('paginationContainer').style.display = 'none';
        } else {
            // 有数据但当前页没有
            tableBody.innerHTML = `
                <tr>
                    <td colspan="7" class="text-center text-base-content/60 py-8">
                        当前页没有数据
                    </td>
                </tr>
            `;
            document.getElementById('paginationContainer').style.display = 'flex';
        }
        return;
    }
    
    elements.emptyState?.classList.add('hidden');
    
    // 渲染分页数据（服务器端分页）
    console.log('[分页调试] 准备渲染域名数量:', domains.length, '域名列表:', domains.map(d => d.name));
    let renderedCount = 0;
    domains.forEach((domain, index) => {
        try {
            const row = createDomainRow(domain);
            tableBody.appendChild(row);
            renderedCount++;
        } catch (error) {
            console.error('[渲染错误] 域名', domain.name, '渲染失败:', error);
        }
    });
    console.log('[分页调试] 实际渲染域名数量:', renderedCount);
    
    // 更新分页信息
    updatePaginationInfo('dashboard', totalFiltered);
}

// 带分页的显示管理域名（服务器端分页）
function displayManageDomainsWithPagination() {
    const tableBody = elements.manageDomainTableBody;
    if (!tableBody) return;
    
    // 清空表格
    tableBody.innerHTML = '';
    
    if (domains.length === 0) {
        if (totalManageDomains === 0) {
            tableBody.innerHTML = `
                <tr>
                    <td colspan="4" class="text-center text-base-content/60 py-8">
                        暂无监控域名
                    </td>
                </tr>
            `;
        } else {
            tableBody.innerHTML = `
                <tr>
                    <td colspan="4" class="text-center text-base-content/60 py-8">
                        当前页没有数据
                    </td>
                </tr>
            `;
        }
        document.getElementById('managePaginationContainer').style.display = 'none';
        return;
    }
    
    // 渲染分页数据（服务器端分页）
    domains.forEach(domain => {
        const row = document.createElement('tr');
        row.className = 'domain-item';
        row.style.backgroundColor = 'transparent';
        const domainName = domain.name || domain;
        // 使用添加时间而不是注册时间
        const addedAt = formatDateTime(domain.added_at);
        row.innerHTML = `
            <td style="background-color: transparent;">${domainName}</td>
            <td style="background-color: transparent;">${addedAt}</td>
            <td style="background-color: transparent;"><span class="badge badge-primary">监控中</span></td>
            <td style="background-color: transparent;">
                <button class="btn btn-sm btn-error" onclick="removeDomain('${domainName}')">
                    删除
                </button>
            </td>
        `;
        
        // 添加hover事件监听器
        row.addEventListener('mouseenter', function() {
            this.style.backgroundColor = '#f3f4f6';
            Array.from(this.children).forEach(td => {
                td.style.backgroundColor = '#f3f4f6';
                td.style.color = '#111827';
            });
        });
        
        row.addEventListener('mouseleave', function() {
            this.style.backgroundColor = 'transparent';
            Array.from(this.children).forEach(td => {
                td.style.backgroundColor = 'transparent';
                td.style.color = 'inherit';
            });
        });
        
        tableBody.appendChild(row);
    });
    
    // 更新分页信息
    updatePaginationInfo('manage', domains.length);
}

// 更新分页信息
function updatePaginationInfo(type, totalFiltered) {
    if (type === 'dashboard') {
        const currentItemCount = domains.length;
        const startIndex = totalFiltered > 0 ? (dashboardCurrentPage - 1) * itemsPerPage + 1 : 0;
        const endIndex = startIndex + currentItemCount - 1;
        const totalPages = Math.ceil(totalFiltered / itemsPerPage);
        
        console.log('[分页调试] 当前页:', dashboardCurrentPage, '每页数量:', itemsPerPage, '当前页项目数:', currentItemCount, '总过滤数:', totalFiltered, '总数:', totalDomains);
        
        document.getElementById('startIndex').textContent = startIndex;
        document.getElementById('endIndex').textContent = endIndex;
        document.getElementById('totalCount').textContent = totalDomains;
        document.getElementById('pageInfo').textContent = `第 ${dashboardCurrentPage} 页`;
        
        document.getElementById('prevPageBtn').disabled = dashboardCurrentPage === 1;
        document.getElementById('nextPageBtn').disabled = dashboardCurrentPage >= totalPages;
        document.getElementById('paginationContainer').style.display = totalFiltered > itemsPerPage ? 'flex' : 'none';
        
    } else if (type === 'manage') {
        const currentItemCount = domains.length;
        const startIndex = totalManageDomains > 0 ? (manageCurrentPage - 1) * itemsPerPage + 1 : 0;
        const endIndex = startIndex + currentItemCount - 1;
        const totalPages = Math.ceil(totalManageDomains / itemsPerPage);
        
        document.getElementById('manageStartIndex').textContent = startIndex;
        document.getElementById('manageEndIndex').textContent = endIndex;
        document.getElementById('manageTotalCount').textContent = totalManageDomains;
        document.getElementById('managePageInfo').textContent = `第 ${manageCurrentPage} 页`;
        
        document.getElementById('managePrevPageBtn').disabled = manageCurrentPage === 1;
        document.getElementById('manageNextPageBtn').disabled = manageCurrentPage >= totalPages;
        document.getElementById('managePaginationContainer').style.display = totalManageDomains > itemsPerPage ? 'flex' : 'none';
    }
}

function domainsDataTotalFiltered(fallback) {
    // 服务器返回 total_filtered，若没有则用当前长度
    if (typeof window.__lastTotalFiltered === 'number') {
        return window.__lastTotalFiltered;
    }
    return fallback;
}

// 移动端dropdown控制
function closeMobileDropdown() {
    const dropdown = document.getElementById('mobile-dropdown');
    const menuBtn = document.getElementById('mobile-menu-btn');
    if (dropdown && menuBtn) {
        // 移除焦点以关闭dropdown
        menuBtn.blur();
        dropdown.blur();
    }
}
// 版本比较函数
function compareVersions(v1, v2) {
    const cleanV1 = v1.replace(/^v/, '');
    const cleanV2 = v2.replace(/^v/, '');
    const parts1 = cleanV1.split('.').map(Number);
    const parts2 = cleanV2.split('.').map(Number);
    for (let i = 0; i < Math.max(parts1.length, parts2.length); i++) {
        const num1 = parts1[i] || 0;
        const num2 = parts2[i] || 0;
        if (num1 > num2) return 1;
        if (num1 < num2) return -1;
    }
    return 0;
}

// 检查更新
async function checkForUpdates() {
    try {
        const response = await fetch('/api/check-update');
        if (!response.ok) {
            console.warn('检查更新失败');
            return;
        }
        const data = await response.json();
        
        // 更新页脚版本号显示
        if (data.currentVersion) {
            const versionElement = document.getElementById('app-version');
            if (versionElement) {
                versionElement.textContent = data.currentVersion;
            }
        }
        
        if (data.error) {
            console.warn('检查更新错误:', data.error);
            return;
        }
        
        // 检查是否在7天内已经提示过
        const lastDismissed = localStorage.getItem('updateDismissed');
        if (lastDismissed) {
            const dismissedTime = new Date(lastDismissed);
            const now = new Date();
            const daysDiff = (now - dismissedTime) / (1000 * 60 * 60 * 24);
            if (daysDiff < 7) {
                console.log('更新提示已在7天内关闭，跳过检查');
                return;
            }
        }
        
        const hasUpdate = compareVersions(data.latestVersion, data.currentVersion) > 0;
        if (hasUpdate) {
            document.getElementById('current-version').textContent = data.currentVersion;
            document.getElementById('latest-version').textContent = data.latestVersion;
            
            // 处理更新日志的换行显示
            const releaseBody = document.getElementById('release-body');
            const announcement = data.announcement || data.body;
            if (releaseBody && announcement) {
                // 将换行符转换为HTML的换行标签
                releaseBody.innerHTML = announcement.replace(/\r\n/g, '<br>').replace(/\n/g, '<br>');
            } else if (releaseBody) {
                releaseBody.textContent = '暂无更新日志';
            }

            if (data.publishedAt) {
                const publishDate = new Date(data.publishedAt);
                document.getElementById('publish-date').textContent = publishDate.toLocaleString('zh-CN', dateTimeOptions);
            }
            const modal = document.getElementById('updateModal');
            modal.showModal();
            document.getElementById('update-dismiss-btn').addEventListener('click', function() {
                localStorage.setItem('updateDismissed', new Date().toISOString());
            }, { once: true });
        }
    } catch (error) {
        console.error('检查更新时发生错误:', error);
    }
}

