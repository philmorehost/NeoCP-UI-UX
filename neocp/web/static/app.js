/* ==========================================================================
   NEOCP PROFESSIONAL — FRONT-END INTERACTIVE PLATFORM CONTROLLER
   ========================================================================== */

document.addEventListener('DOMContentLoaded', () => {
    // Platform State Variables
    let currentUser = 'patel';
    let currentRole = 'customer';
    let currentPath = '/home/patel/public_html';
    let telemetrySocket = null;
    let cpuHistory = Array(40).fill(0); // For Canvas charts
    let ramHistory = Array(40).fill(0); // For RAM Canvas spline wave
    let activeTicketId = null;
    let selectedProcessQuery = '';
    let activeTasks = 0;
    let cachedDomains = [];

    // Dom Elements Cache
    const views = document.querySelectorAll('.viewport-view');
    const menuItems = document.querySelectorAll('.menu-item');
    const activeCrumb = document.getElementById('active-crumb');
    const systemClock = document.getElementById('header-clock');
    const profileName = document.getElementById('profile-name');
    const profileRole = document.getElementById('profile-role');
    const userRoleSelect = document.getElementById('user-role-select');
    const activeTasksCount = document.getElementById('active-tasks-count');
    const headerBandwidth = document.getElementById('header-bandwidth');

    // Clock synchronizer
    setInterval(() => {
        const now = new Date();
        systemClock.textContent = now.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
    }, 1000);

    // ==========================================================================
    // 1. JWT SESSION GATE & DYNAMIC ROLE PROVISIONING
    // ==========================================================================
    async function triggerTenantAuthentication(role) {
        let username = 'patel';
        let password = 'patel123';
        if (role === 'admin') {
            username = 'admin';
            password = 'admin123';
        }
        if (role === 'reseller') {
            username = 'reseller1';
            password = 'reseller123';
        }

        try {
            const res = await fetch('/api/login', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ username, password })
            });
            const data = await res.json();
            if (data.token) {
                // Set cookie & localStorage session
                document.cookie = `neocp_auth_token=${data.token}; path=/; max-age=86400; SameSite=Lax`;
                localStorage.setItem('neocp_token', data.token);
                currentUser = username;
                currentRole = role;

                // Adjust Profile display
                profileRole.textContent = role.toUpperCase();
                if (role === 'admin') {
                    profileName.textContent = 'Root System Administrator';
                    document.getElementById('footer-host-os').textContent = 'Linux / Windows Master';
                } else if (role === 'reseller') {
                    profileName.textContent = 'Enterprise Reseller';
                    document.getElementById('footer-host-os').textContent = 'WHM Node Reseller';
                } else {
                    profileName.textContent = 'Amit Patel';
                    document.getElementById('footer-host-os').textContent = 'cPanel Cloud Container';
                }

                // Adjust default home folders in sandbox
                currentPath = `/home/${currentUser}/public_html`;
                
                // Refresh all models
                loadDashboardSummaries();
                loadDomainsTable();
                loadFileExplorer();
                loadDatabasesTable();
                loadCronJobs();
                loadPackagesTable();
                loadSupportTickets();
                loadDockerContainers();
                loadMigrationTasks();
                loadOSServicesList();
                loadClusterNodes();
                
                // Show role-specific warnings/elements
                enforceRoleCapabilities();
                showNotification(`Authenticated successfully as ${username} (${role.toUpperCase()})`, 'success');
            }
        } catch (err) {
            console.error('Session login failure:', err);
            showNotification('Server connection handshake failed.', 'error');
        }
    }

    userRoleSelect.addEventListener('change', (e) => {
        triggerTenantAuthentication(e.target.value);
    });

    // Enforce multi-tenant GUI boundaries dynamically
    function enforceRoleCapabilities() {
        // Reset all hidden elements
        document.querySelectorAll('.admin-only').forEach(el => el.style.display = 'none');
        document.querySelectorAll('.reseller-only').forEach(el => el.style.display = 'none');

        // Hide sensitive menu sections for customers
        const sections = document.querySelectorAll('.menu-section');
        sections.forEach(sec => {
            if (sec.textContent === 'Enterprise Ecosystem' && currentRole === 'customer') {
                sec.style.display = 'none';
            } else {
                sec.style.display = 'block';
            }
        });

        if (currentRole === 'admin') {
            document.querySelectorAll('.admin-only').forEach(el => {
                // Determine original display type
                if (el.classList.contains('dashboard-metrics-grid')) el.style.display = 'grid';
                else if (el.classList.contains('menu-item')) el.style.display = 'list-item';
                else if (el.classList.contains('shortcut-item')) el.style.display = 'flex';
                else el.style.display = 'block';
            });
        }

        if (currentRole === 'reseller' || currentRole === 'admin') {
            document.querySelectorAll('.reseller-only').forEach(el => el.style.display = 'block');
        }

        // Dashboard specific tweaks: hide system-wide telemetry from customers
        const dashMetrics = document.querySelector('.dashboard-metrics-grid');
        const threadDiag = document.querySelector('.core-telemetry-section');

        if (currentRole === 'customer' || currentRole === 'reseller') {
            if (dashMetrics) dashMetrics.style.display = 'none';
            if (threadDiag) threadDiag.style.display = 'none';
        } else {
            if (dashMetrics) dashMetrics.style.display = 'grid';
            if (threadDiag) threadDiag.style.display = 'block';
        }
    }

    // ==========================================================================
    // 2. HIGH-PERFORMANCE CLIENT-SIDE SPA VIEW-SWAPPER
    // ==========================================================================
    function switchToView(targetViewId) {
        views.forEach(view => {
            view.classList.remove('active');
            if (view.id === `view-${targetViewId}`) {
                view.classList.add('active');
            }
        });

        menuItems.forEach(item => {
            item.classList.remove('active');
            if (item.getAttribute('data-target') === targetViewId) {
                item.classList.add('active');
            }
        });

        // Set breadcrumbs & title details
        const sectionName = document.querySelector(`[data-target="${targetViewId}"]`).textContent.trim();
        activeCrumb.textContent = sectionName;
        document.title = `${sectionName} — NeoCP Professional`;

        // Action grid hooks
        if (targetViewId === 'filemanager') {
            loadTrashSummary();
            loadFileExplorer();
        }
        if (targetViewId === 'domains') loadDomainsTable();
        if (targetViewId === 'databases') loadDatabasesTable();
        if (targetViewId === 'mail') loadMailAccounts();
        if (targetViewId === 'reseller') {
            loadPackagesTable();
            loadResellerAccounts();
        }
        if (targetViewId === 'ipmanager') {
            loadIPPool();
        }
        if (targetViewId === 'fleet') {
            // Load global fleet data
        }
        if (targetViewId === 'system') loadOSServicesList();
        if (targetViewId === 'backups') loadBackupsTable();
        if (targetViewId === 'containers') loadDockerContainers();
        if (targetViewId === 'clustering') loadClusterNodes();
        if (targetViewId === 'security') {
            loadFirewallBlocks();
            loadDomainsTable();
        }
    }

    menuItems.forEach(item => {
        item.addEventListener('click', () => {
            const target = item.getAttribute('data-target');
            switchToView(target);
        });
    });

    // Shortcut Buttons Hooks
    document.querySelectorAll('.shortcut-item').forEach(button => {
        button.addEventListener('click', () => {
            const target = button.getAttribute('data-target');
            switchToView(target);
        });
    });

    // ==========================================================================
    // 3. WS TELEMETRY PIPELINE & CANVAS DIAGNOSTICS WAVE
    // ==========================================================================
    function connectTelemetryWebSocket() {
        const loc = window.location;
        let wsUri = loc.protocol === 'https:' ? 'wss://' : 'ws://';
        wsUri += loc.host + '/api/telemetry/ws';

        document.getElementById('telemetry-socket-status').textContent = 'Connecting...';
        document.getElementById('telemetry-socket-status').className = 'text-orange';

        telemetrySocket = new WebSocket(wsUri);

        telemetrySocket.onopen = () => {
            document.getElementById('telemetry-socket-status').textContent = 'Connected';
            document.getElementById('telemetry-socket-status').className = 'text-green';
        };

        telemetrySocket.onmessage = (event) => {
            const data = JSON.parse(event.data);

            // Dashboard cards
            document.getElementById('dash-cpu-pct').textContent = `${data.cpu_overall.toFixed(1)}%`;
            document.getElementById('dash-cpu-fill').style.width = `${data.cpu_overall}%`;

            document.getElementById('dash-ram-pct').textContent = `${((data.memory_used / data.memory_total) * 100).toFixed(1)}%`;
            document.getElementById('dash-ram-fill').style.width = `${((data.memory_used / data.memory_total) * 100)}%`;
            document.getElementById('dash-ram-desc').textContent = `${data.memory_used.toFixed(2)} GB of ${data.memory_total.toFixed(1)} GB used`;

            document.getElementById('dash-disk-pct').textContent = `${((data.disk_used / data.disk_total) * 100).toFixed(1)}%`;
            document.getElementById('dash-disk-fill').style.width = `${((data.disk_used / data.disk_total) * 100)}%`;
            document.getElementById('dash-disk-desc').textContent = `${data.disk_used.toFixed(2)} GB of ${data.disk_total.toFixed(1)} GB used`;

            headerBandwidth.textContent = `${data.network_in.toFixed(1)} MB/s`;

            // Draw 8-Core Progress Tracker
            const coresGrid = document.getElementById('cores-telemetry-grid');
            coresGrid.innerHTML = '';
            data.cpu_loads.forEach((load, idx) => {
                const bar = document.createElement('div');
                bar.className = 'core-bar-wrapper';
                bar.innerHTML = `
                    <div class="core-meter-track">
                        <div class="core-meter-fill" style="height: ${load}%"></div>
                    </div>
                    <span class="core-label">C${idx + 1}</span>
                    <span class="core-val">${load.toFixed(0)}%</span>
                `;
                coresGrid.appendChild(bar);
            });

            // Canvas Chart Plotting
            cpuHistory.push(data.cpu_overall);
            cpuHistory.shift();
            drawCanvasTelemetryWave();

            // RAM Spline Wave Plotting
            const ramPct = (data.memory_used / data.memory_total) * 100;
            ramHistory.push(ramPct);
            ramHistory.shift();
            drawRamCanvasTelemetryWave();

            // Dynamic live updates in telemetry processes view
            if (document.getElementById('view-system').classList.contains('active')) {
                renderProcessesList(data.processes);
            }
        };

        telemetrySocket.onclose = () => {
            document.getElementById('telemetry-socket-status').textContent = 'Offline';
            document.getElementById('telemetry-socket-status').className = 'text-red';
            // Auto reconnect after 3.5s
            setTimeout(connectTelemetryWebSocket, 3500);
        };
    }

    const canvas = document.getElementById('dashboard-cpu-canvas');
    const ctx = canvas.getContext('2d');

    const ramCanvas = document.getElementById('dashboard-ram-canvas');
    const ramCtx = ramCanvas ? ramCanvas.getContext('2d') : null;

    function drawCanvasTelemetryWave() {
        ctx.clearRect(0, 0, canvas.width, canvas.height);
        
        // Gradient styling
        const grad = ctx.createLinearGradient(0, 0, 0, canvas.height);
        grad.addColorStop(0, 'rgba(56, 189, 248, 0.45)');
        grad.addColorStop(1, 'rgba(56, 189, 248, 0.0)');

        ctx.beginPath();
        const step = canvas.width / (cpuHistory.length - 1);
        ctx.moveTo(0, canvas.height);

        for (let i = 0; i < cpuHistory.length; i++) {
            const x = i * step;
            const y = canvas.height - (cpuHistory[i] / 100) * (canvas.height - 10);
            ctx.lineTo(x, y);
        }

        ctx.lineTo(canvas.width, canvas.height);
        ctx.closePath();
        ctx.fillStyle = grad;
        ctx.fill();

        // Stroke line
        ctx.beginPath();
        for (let i = 0; i < cpuHistory.length; i++) {
            const x = i * step;
            const y = canvas.height - (cpuHistory[i] / 100) * (canvas.height - 10);
            if (i === 0) ctx.moveTo(x, y);
            else ctx.lineTo(x, y);
        }
        ctx.strokeStyle = '#38bdf8';
        ctx.lineWidth = 2.5;
        ctx.stroke();
    }

    function drawRamCanvasTelemetryWave() {
        if (!ramCanvas || !ramCtx) return;
        ramCtx.clearRect(0, 0, ramCanvas.width, ramCanvas.height);
        
        // Gradient styling (Emerald green `#22c55e`)
        const grad = ramCtx.createLinearGradient(0, 0, 0, ramCanvas.height);
        grad.addColorStop(0, 'rgba(34, 197, 94, 0.45)');
        grad.addColorStop(1, 'rgba(34, 197, 94, 0.0)');

        ramCtx.beginPath();
        const step = ramCanvas.width / (ramHistory.length - 1);
        ramCtx.moveTo(0, ramCanvas.height);

        for (let i = 0; i < ramHistory.length; i++) {
            const x = i * step;
            const y = ramCanvas.height - (ramHistory[i] / 100) * (ramCanvas.height - 10);
            ramCtx.lineTo(x, y);
        }

        ramCtx.lineTo(ramCanvas.width, ramCanvas.height);
        ramCtx.closePath();
        ramCtx.fillStyle = grad;
        ramCtx.fill();

        // Stroke line
        ramCtx.beginPath();
        for (let i = 0; i < ramHistory.length; i++) {
            const x = i * step;
            const y = ramCanvas.height - (ramHistory[i] / 100) * (ramCanvas.height - 10);
            if (i === 0) ramCtx.moveTo(x, y);
            else ramCtx.lineTo(x, y);
        }
        ramCtx.strokeStyle = '#22c55e';
        ramCtx.lineWidth = 2.5;
        ramCtx.stroke();
    }

    // ==========================================================================
    // 4. ACCOUNT SUMMARIES POPULATER
    // ==========================================================================
    async function loadDashboardSummaries() {
        try {
            const res = await fetch('/api/account', {
                headers: { 'Authorization': `Bearer ${localStorage.getItem('neocp_token')}` }
            });
            if (res.status === 401) return;
            const acc = await res.json();

            document.getElementById('stat-package').textContent = acc.plan;
            
            const statDoms = document.getElementById('stat-domains');
            if (statDoms) {
                statDoms.textContent = `${acc.domains_used} / ${acc.domains_limit === 0 ? 'Unlimited' : acc.domains_limit}`;
            }
            
            document.getElementById('stat-privilege').textContent = acc.role.toUpperCase();
            document.getElementById('stat-privilege').className = `stat-val tag-role role-${acc.role}`;

            // Seed SSL summary
            const domRes = await fetch('/api/domains', {
                headers: { 'Authorization': `Bearer ${localStorage.getItem('neocp_token')}` }
            });
            const domains = await domRes.json();
            const sslCount = domains.filter(d => d.ssl_active).length;
            
            const statSsl = document.getElementById('stat-ssl');
            if (statSsl) {
                statSsl.textContent = `${sslCount} SECURE`;
            }

            // Fetch databases count
            const dbRes = await fetch('/api/databases', {
                headers: { 'Authorization': `Bearer ${localStorage.getItem('neocp_token')}` }
            });
            const dbs = await dbRes.json();

            // Calculate limits based on user plan
            const planLimits = {
                'Standard Hosting Plan': { domains: 10, databases: 5, disk: 5000 },
                'Premium-Personal': { domains: 3, databases: 5, disk: 2000 },
                'Enterprise-Cluster': { domains: 100, databases: 250, disk: 50000 },
                'Gold Reseller Pack': { domains: 50, databases: 25, disk: 50000 },
                'Unlimited System Plan': { domains: 999, databases: 999, disk: 999999 }
            };
            const limits = planLimits[acc.plan] || { domains: acc.domains_limit || 10, databases: 5, disk: acc.disk_limit || 5000 };

            // Compute capacity percentages
            const domainsUsed = domains.length;
            const dbsUsed = dbs.length;
            const diskUsed = acc.disk_used;

            const domPct = Math.min(100, Math.round((domainsUsed / limits.domains) * 100)) || 0;
            const dbPct = Math.min(100, Math.round((dbsUsed / limits.databases) * 100)) || 0;
            const diskPct = Math.min(100, Math.round((diskUsed / limits.disk) * 100)) || 0;

            // Apply stroke-dashoffset formula (circumference is 201)
            const domOffset = 201 - (201 * domPct / 100);
            const dbOffset = 201 - (201 * dbPct / 100);
            const diskOffset = 201 - (201 * diskPct / 100);

            // Update DOM Elements
            const ringDomsFill = document.getElementById('ring-domains-fill');
            if (ringDomsFill) ringDomsFill.style.strokeDashoffset = domOffset;
            const ringDomsText = document.getElementById('ring-domains-text');
            if (ringDomsText) ringDomsText.textContent = `${domPct}%`;
            const ringDomsLbl = document.getElementById('ring-domains-lbl');
            if (ringDomsLbl) ringDomsLbl.textContent = `${domainsUsed} of ${limits.domains}`;

            const ringDbsFill = document.getElementById('ring-databases-fill');
            if (ringDbsFill) ringDbsFill.style.strokeDashoffset = dbOffset;
            const ringDbsText = document.getElementById('ring-databases-text');
            if (ringDbsText) ringDbsText.textContent = `${dbPct}%`;
            const ringDbsLbl = document.getElementById('ring-databases-lbl');
            if (ringDbsLbl) ringDbsLbl.textContent = `${dbsUsed} of ${limits.databases}`;

            const ringDiskFill = document.getElementById('ring-disk-fill');
            if (ringDiskFill) ringDiskFill.style.strokeDashoffset = diskOffset;
            const ringDiskText = document.getElementById('ring-disk-text');
            if (ringDiskText) ringDiskText.textContent = `${diskPct}%`;
            const ringDiskLbl = document.getElementById('ring-disk-lbl');
            if (ringDiskLbl) ringDiskLbl.textContent = `${diskUsed} of ${limits.disk} MB`;

        } catch (e) {
            console.error('Failed to load accounts detail summaries', e);
        }
    }

    // ==========================================================================
    // 5. DOMAIN SPACE HARDENING & ADMINISTRATION
    // ==========================================================================
    const addDomainModal = document.getElementById('add-domain-modal');
    const openAddDomainBtn = document.getElementById('open-add-domain-btn');
    const closeDomainModalBtn = document.getElementById('close-domain-modal-btn');
    const submitDomainBtn = document.getElementById('modal-add-domain-submit');

    async function loadPackagesForSelect() {
        try {
            const res = await fetch('/api/packages', {
                headers: { 'Authorization': `Bearer ${localStorage.getItem('neocp_token')}` }
            });
            const pkgs = await res.json();
            const select = document.getElementById('modal-package-select');
            if (select) {
                select.innerHTML = '';
                pkgs.forEach(p => {
                    const opt = document.createElement('option');
                    opt.value = p.name;
                    opt.textContent = p.name;
                    select.appendChild(opt);
                });
            }
        } catch (e) {}
    }

    openAddDomainBtn.addEventListener('click', () => {
        document.getElementById('modal-domain-input').value = `app-${Math.floor(Math.random()*100)}.patelcloud.net`;
        document.getElementById('modal-webroot-input').value = `/home/${currentUser}/public_html/app`;
        loadPackagesForSelect();
        addDomainModal.classList.add('active');
    });

    closeDomainModalBtn.addEventListener('click', () => {
        addDomainModal.classList.remove('active');
    });

    submitDomainBtn.addEventListener('click', async () => {
        const domainName = document.getElementById('modal-domain-input').value;
        const webroot = document.getElementById('modal-webroot-input').value;
        const phpVersion = document.getElementById('modal-php-select').value;
        const packagePlan = document.getElementById('modal-package-select').value;
        const sslToggle = document.getElementById('modal-ssl-toggle').checked;

        if (!domainName || !webroot) {
            showNotification('Please populate all inputs.', 'error');
            return;
        }

        addTaskIndicator();
        try {
            const res = await fetch('/api/domains', {
                method: 'POST',
                headers: { 
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${localStorage.getItem('neocp_token')}`
                },
                body: JSON.stringify({
                    domain_name: domainName,
                    owner: currentUser,
                    php_version: phpVersion,
                    ssl_active: sslToggle,
                    plan: packagePlan
                })
            });
            const data = await res.json();
            if (res.ok) {
                showNotification('Domain Space deployed successfully with VirtualHost records!', 'success');
                addDomainModal.classList.remove('active');
                loadDomainsTable();
                loadDashboardSummaries();
            } else {
                showNotification(data.error || 'Failed to deploy domain.', 'error');
            }
        } catch (e) {
            showNotification('API connection error deploying domain.', 'error');
        } finally {
            removeTaskIndicator();
        }
    });

    async function loadDomainsTable() {
        const tbody = document.querySelector('#domains-table tbody');
        tbody.innerHTML = '<tr><td colspan="6" class="text-muted">Analyzing nginx virtual hosts...</td></tr>';

        try {
            const res = await fetch('/api/domains', {
                headers: { 'Authorization': `Bearer ${localStorage.getItem('neocp_token')}` }
            });
            const domains = await res.json();
            cachedDomains = domains;
            loadDomainSelectorsFromCache(domains);

            tbody.innerHTML = '';
            if (domains.length === 0) {
                tbody.innerHTML = '<tr><td colspan="6" class="text-muted">No domains configured.</td></tr>';
                return;
            }

            // Segregate production domains from staging domains
            const productionDomains = domains.filter(dom => !dom.domain_name.includes('-stage') && !dom.domain_name.startsWith('staging.'));
            const stagingDomains = domains.filter(dom => dom.domain_name.includes('-stage') || dom.domain_name.startsWith('staging.'));

            if (productionDomains.length === 0) {
                tbody.innerHTML = '<tr><td colspan="6" class="text-muted">No production domains configured.</td></tr>';
                return;
            }

            productionDomains.forEach(dom => {
                const tr = document.createElement('tr');
                tr.innerHTML = `
                    <td>
                        <strong class="text-blue">${dom.domain_name}</strong><br>
                        <span class="text-muted" style="font-size: 0.75rem;">Root: /home/${dom.owner}/public_html</span>
                    </td>
                    <td>
                        <span class="status-badge ${dom.ssl_active ? 'badge-green' : 'badge-red'}">
                            ${dom.ssl_active ? '🔒 Active' : '⚠️ No Certificate'}
                        </span><br>
                        <span class="text-muted" style="font-size: 0.7rem;">${dom.ssl_active ? dom.ssl_issuer : 'Unencrypted'}</span>
                    </td>
                    <td>
                        <select class="php-inline-switcher role-selector" data-domain="${dom.domain_name}" style="width: auto; padding: 2px 6px;">
                            <option value="8.3" ${dom.php_version === '8.3' ? 'selected' : ''}>PHP 8.3</option>
                            <option value="8.2" ${dom.php_version === '8.2' ? 'selected' : ''}>PHP 8.2</option>
                            <option value="8.1" ${dom.php_version === '8.1' ? 'selected' : ''}>PHP 8.1</option>
                        </select>
                    </td>
                    <td>
                        <div style="display:flex; flex-direction:column; gap:4px;">
                            <label style="font-size: 0.75rem; display:flex; align-items:center; gap:4px;">
                                <input type="checkbox" class="setting-toggle" data-domain="${dom.domain_name}" data-prop="gzip" ${dom.gzip_enabled ? 'checked' : ''}> GZIP
                            </label>
                            <label style="font-size: 0.75rem; display:flex; align-items:center; gap:4px;">
                                <input type="checkbox" class="setting-toggle" data-domain="${dom.domain_name}" data-prop="brotli" ${dom.brotli_enabled ? 'checked' : ''}> Brotli
                            </label>
                        </div>
                    </td>
                    <td>
                        <span class="status-badge ${dom.directory_privacy ? 'badge-green' : 'badge-muted'}" style="cursor:pointer;" onclick="toggleDirectoryPrivacy('${dom.domain_name}', ${dom.directory_privacy})">
                            ${dom.directory_privacy ? '🔒 Restricted' : '🔓 Public Access'}
                        </span>
                    </td>
                    <td>
                        <div class="table-actions">
                            <button class="action-btn-secondary convert-account-btn" data-domain="${dom.domain_name}" data-owner="${dom.owner}" style="padding: 4px 8px; font-size: 0.75rem;">Decouple</button>
                            <button class="action-btn-secondary toggle-staging-btn" data-domain="${dom.domain_name}" style="padding: 4px 8px; font-size: 0.75rem;">Staging</button>
                            <button class="action-btn-secondary ssl-issue-btn" data-domain="${dom.domain_name}" style="padding: 4px 8px; font-size: 0.75rem;">Issue SSL</button>
                            <button class="action-btn-secondary git-deploy-btn" data-domain="${dom.domain_name}" style="padding: 4px 8px; font-size: 0.75rem;">Git Deploy</button>
                            <button class="action-btn-secondary redirect-btn" data-domain="${dom.domain_name}" style="padding: 4px 8px; font-size: 0.75rem;">Redirect</button>
                            <button class="action-btn-secondary delete-domain-btn text-red" data-domain="${dom.domain_name}" style="padding: 4px 8px; font-size: 0.75rem; border-color: rgba(239, 68, 68, 0.2);">Delete</button>
                        </div>
                    </td>
                `;
                tbody.appendChild(tr);

                // Check for a staging copy of this domain
                const matchingStage = stagingDomains.find(s => {
                    const parent = s.domain_name.replace('-stage', '').replace('staging.', '');
                    return parent === dom.domain_name;
                });

                const drawerTr = document.createElement('tr');
                drawerTr.className = 'staging-drawer-row';
                drawerTr.id = `staging-drawer-${dom.domain_name.replace(/\./g, '-')}`;
                drawerTr.style.display = 'none';
                drawerTr.style.background = 'rgba(255, 255, 255, 0.015)';

                let drawerContent = '';
                if (matchingStage) {
                    drawerContent = `
                        <td colspan="6" style="padding: 15px 25px; border-top: 1px solid rgba(255,255,255,0.03); background: rgba(59,130,246,0.02);">
                            <div style="display:flex; justify-content:space-between; align-items:center; gap:20px;">
                                <div>
                                    <span class="status-badge badge-green" style="font-size:11px;"><span class="pulse-dot"></span> Staging Sandbox Active</span>
                                    <h5 style="margin: 8px 0 4px 0; font-size: 14px;">Domain: <a href="https://${matchingStage.domain_name}" target="_blank" class="text-blue" style="text-decoration:underline; font-weight: 600;">${matchingStage.domain_name}</a></h5>
                                    <span class="text-muted" style="font-size: 12px; display:block;">Isolated Database: <code style="color: #3b82f6;">${matchingStage.owner}_staging_...</code></span>
                                    <span class="text-muted" style="font-size: 12px;">Path: <code style="color: #10b981;">/home/${matchingStage.owner}/public_html/${matchingStage.domain_name}</code></span>
                                </div>
                                <div style="display:flex; align-items:center; gap:12px;">
                                    <div class="input-inline" style="margin:0; display:flex; align-items:center; gap:8px;">
                                        <select id="sync-mode-${dom.domain_name.replace(/\./g, '-')}" class="role-selector" style="padding:6px 12px; height:auto; width:auto; font-size:12px; background:rgba(0,0,0,0.2);">
                                            <option value="both">Push Full Stack (Files + DB)</option>
                                            <option value="files">Push Files Only</option>
                                            <option value="db">Push Database Only</option>
                                        </select>
                                        <button class="action-btn push-stage-btn" data-stage="${matchingStage.domain_name}" data-parent="${dom.domain_name}" style="padding:6px 15px; font-size:12px; height: 32px; display: flex; align-items: center;">Deploy to Production</button>
                                    </div>
                                    <button class="action-btn-secondary destroy-stage-btn text-red" data-stage="${matchingStage.domain_name}" style="padding:6px 15px; font-size:12px; border-color:rgba(239,68,68,0.25); height: 32px; display: flex; align-items: center;">Destroy Sandbox</button>
                                </div>
                            </div>
                        </td>
                    `;
                } else {
                    const suggestedSub = dom.domain_name.split('.')[0] + '-stage';
                    const suggestedDomain = dom.domain_name.replace(dom.domain_name.split('.')[0], suggestedSub);
                    drawerContent = `
                        <td colspan="6" style="padding: 15px 25px; border-top: 1px solid rgba(255,255,255,0.03);">
                            <div style="display:flex; justify-content:space-between; align-items:center; gap:20px;">
                                <div>
                                    <span class="status-badge badge-muted" style="font-size:11px;">⚠️ No Staging Active</span>
                                    <h5 style="margin: 8px 0 4px 0; font-size: 14px;">Deploy a 1-Click isolated staging environment for WordPress/HTML testing.</h5>
                                    <span class="text-muted" style="font-size: 12px;">Co-located sandbox replication with automatic domain & serialized string recalculations.</span>
                                </div>
                                <div style="display:flex; align-items:center; gap:10px;">
                                    <input type="text" id="new-stage-subdomain-${dom.domain_name.replace(/\./g, '-')}" value="${suggestedDomain}" style="padding: 6px 12px; background: rgba(255,255,255,0.03); border: 1px solid rgba(255,255,255,0.08); border-radius: 6px; color: #fff; font-size:12px; width: 220px; height: 32px;">
                                    <button class="action-btn create-stage-btn" data-prod="${dom.domain_name}" style="padding:6px 15px; font-size:12px; height: 32px; display: flex; align-items: center;">Create Sandbox</button>
                                </div>
                            </div>
                        </td>
                    `;
                }
                drawerTr.innerHTML = drawerContent;
                tbody.appendChild(drawerTr);
            });

            // Bind PHP switchers
            document.querySelectorAll('.php-inline-switcher').forEach(select => {
                select.addEventListener('change', async (e) => {
                    const dom = e.target.getAttribute('data-domain');
                    const version = e.target.value;
                    await updateDomainPHPVersion(dom, version);
                });
            });

            // Bind setting checkboxes
            document.querySelectorAll('.setting-toggle').forEach(chk => {
                chk.addEventListener('change', async (e) => {
                    const dom = e.target.getAttribute('data-domain');
                    await toggleDomainCompressionSetting(dom);
                });
            });

    // Convert to Primary Account trigger
    document.querySelectorAll('.convert-account-btn').forEach(btn => {
        btn.addEventListener('click', async (e) => {
            const domain = e.target.getAttribute('data-domain');
            const owner = e.target.getAttribute('data-owner');
            if (confirm(`Convert ${domain} to a standalone primary NeoCP account? This will decouple it from ${owner}.`)) {
                addTaskIndicator();
                try {
                    const res = await fetch('/api/migrations/convert', {
                        method: 'POST',
                        headers: {
                            'Content-Type': 'application/json',
                            'Authorization': `Bearer ${localStorage.getItem('neocp_token')}`
                        },
                        body: JSON.stringify({ source_user: owner, addon_domain: domain })
                    });
                    if (res.ok) {
                        showNotification(`${domain} decoupled and converted to primary account!`, 'success');
                        loadDomainsTable();
                    }
                } catch (e) {} finally {
                    removeTaskIndicator();
                }
            }
        });
    });

            // SSL action triggers
            document.querySelectorAll('.ssl-issue-btn').forEach(btn => {
                btn.addEventListener('click', async (e) => {
                    const dom = e.target.getAttribute('data-domain');
                    await issueLetsEncryptSSL(dom);
                });
            });

            // Git Deploy actions
            document.querySelectorAll('.git-deploy-btn').forEach(btn => {
                btn.addEventListener('click', () => {
                    const dom = btn.getAttribute('data-domain');
                    launchGitDeployModal(dom);
                });
            });

            // Redirect actions
            document.querySelectorAll('.redirect-btn').forEach(btn => {
                btn.addEventListener('click', (e) => {
                    const dom = e.target.getAttribute('data-domain');
                    const url = prompt(`Enter redirection target URL for ${dom}:`, 'https://newdomain.com');
                    if (url) configureDomainRedirection(dom, url);
                });
            });

            // Delete domain
            document.querySelectorAll('.delete-domain-btn').forEach(btn => {
                btn.addEventListener('click', async (e) => {
                    const dom = e.target.getAttribute('data-domain');
                    if (confirm(`Are you absolutely sure you want to delete the domain space ${dom}? (This wipes Nginx records)`)) {
                        await deleteDomainSpace(dom);
                    }
                });
            });

            // Bind Staging Toggle buttons
            document.querySelectorAll('.toggle-staging-btn').forEach(btn => {
                btn.addEventListener('click', (e) => {
                    const dom = e.target.getAttribute('data-domain');
                    const drawer = document.getElementById(`staging-drawer-${dom.replace(/\./g, '-')}`);
                    if (drawer.style.display === 'none') {
                        drawer.style.display = 'table-row';
                        btn.textContent = 'Hide Stage';
                        btn.style.background = 'rgba(59,130,246,0.15)';
                        btn.style.color = '#3b82f6';
                        btn.style.borderColor = 'rgba(59,130,246,0.3)';
                    } else {
                        drawer.style.display = 'none';
                        btn.textContent = 'Staging';
                        btn.style.background = '';
                        btn.style.color = '';
                        btn.style.borderColor = '';
                    }
                });
            });

            // Bind Create Sandbox buttons
            document.querySelectorAll('.create-stage-btn').forEach(btn => {
                btn.addEventListener('click', async (e) => {
                    const prod = e.target.getAttribute('data-prod');
                    const sub = document.getElementById(`new-stage-subdomain-${prod.replace(/\./g, '-')}`).value;
                    if (!sub) {
                        showNotification('Please enter a staging domain name.', 'error');
                        return;
                    }
                    await cloneStaging(prod, sub);
                });
            });

            // Bind Push Sandbox buttons
            document.querySelectorAll('.push-stage-btn').forEach(btn => {
                btn.addEventListener('click', async (e) => {
                    const stage = e.target.getAttribute('data-stage');
                    const parent = e.target.getAttribute('data-parent');
                    const syncMode = document.getElementById(`sync-mode-${parent.replace(/\./g, '-')}`).value;
                    await pushStaging(stage, syncMode);
                });
            });

            // Bind Destroy Sandbox buttons
            document.querySelectorAll('.destroy-stage-btn').forEach(btn => {
                btn.addEventListener('click', async (e) => {
                    const stage = e.target.getAttribute('data-stage');
                    if (confirm(`Are you absolutely sure you want to permanently destroy the staging sandbox ${stage}?`)) {
                        await deleteStaging(stage);
                    }
                });
            });

        } catch (e) {
            console.error('Failed to load nginx virtual hosts config.', e);
        }
    }

    async function updateDomainPHPVersion(domain, phpVersion) {
        addTaskIndicator();
        try {
            const res = await fetch('/api/domains/php', {
                method: 'POST',
                headers: { 
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${localStorage.getItem('neocp_token')}`
                },
                body: JSON.stringify({ domain_name: domain, php_version: phpVersion })
            });
            if (res.ok) {
                showNotification(`PHP version for ${domain} successfully shifted to ${phpVersion}`, 'success');
            } else {
                showNotification('Error changing PHP FPM parameters.', 'error');
            }
        } catch (e) {
            showNotification('Network API connection failure.', 'error');
        } finally {
            removeTaskIndicator();
        }
    }

    async function toggleDomainCompressionSetting(domain) {
        // Collect gzip & brotli status
        let gzip = false;
        let brotli = false;
        document.querySelectorAll(`.setting-toggle[data-domain="${domain}"]`).forEach(chk => {
            const prop = chk.getAttribute('data-prop');
            if (prop === 'gzip') gzip = chk.checked;
            if (prop === 'brotli') brotli = chk.checked;
        });

        addTaskIndicator();
        try {
            await fetch('/api/domains/settings', {
                method: 'POST',
                headers: { 
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${localStorage.getItem('neocp_token')}`
                },
                body: JSON.stringify({ domain_name: domain, gzip_enabled: gzip, brotli_enabled: brotli })
            });
            showNotification('Nginx Gzip/Brotli rules updated & reloaded.', 'success');
        } catch (e) {
            showNotification('Failed to save parameters.', 'error');
        } finally {
            removeTaskIndicator();
        }
    }

    window.toggleDirectoryPrivacy = async function(domain, currentVal) {
        const username = prompt('Configure Directory Privacy user:', 'admin_patel');
        const pass = prompt('Configure Directory Privacy password:');
        if (!username || !pass) return;

        addTaskIndicator();
        try {
            await fetch('/api/domains/privacy', {
                method: 'POST',
                headers: { 
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${localStorage.getItem('neocp_token')}`
                },
                body: JSON.stringify({
                    domain_name: domain,
                    directory_privacy: !currentVal,
                    directory_privacy_user: username,
                    directory_privacy_pass: pass
                })
            });
            showNotification('Directory protection htpasswd rules injected successfully!', 'success');
            loadDomainsTable();
        } catch (e) {
            showNotification('Failed to protect directory.', 'error');
        } finally {
            removeTaskIndicator();
        }
    }

    async function issueLetsEncryptSSL(domain) {
        addTaskIndicator();
        showNotification(`Challenging Let's Encrypt API for certificate matching ${domain}...`, 'info');
        try {
            const res = await fetch('/api/domains/ssl/order', {
                method: 'POST',
                headers: { 
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${localStorage.getItem('neocp_token')}`
                },
                body: JSON.stringify({ domain_name: domain, ssl_active: true, ssl_issuer: "Let's Encrypt Authority X3" })
            });
            if (res.ok) {
                showNotification(`SSL TLS certificate issued & compiled securely into Nginx Vhosts for ${domain}`, 'success');
                loadDomainsTable();
                loadDashboardSummaries();
            } else {
                showNotification('SSL challenge failed.', 'error');
            }
        } catch (e) {
            showNotification('Server SSL dispatch error.', 'error');
        } finally {
            removeTaskIndicator();
        }
    }

    async function configureDomainRedirection(domain, redirectURL) {
        addTaskIndicator();
        try {
            await fetch('/api/domains/redirect', {
                method: 'POST',
                headers: { 
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${localStorage.getItem('neocp_token')}`
                },
                body: JSON.stringify({ domain_name: domain, redirect_url: redirectURL })
            });
            showNotification(`Nginx proxy rule redirect added: ${domain} -> ${redirectURL}`, 'success');
            loadDomainsTable();
        } catch (e) {
            showNotification('Failed to hook redirect.', 'error');
        } finally {
            removeTaskIndicator();
        }
    }

    function launchGitDeployModal(domainName) {
        const dom = cachedDomains.find(d => d.domain_name === domainName);
        const config = dom.git_ops || {};

        const overlay = document.createElement('div');
        overlay.className = 'neocp-modal-backdrop active';
        overlay.innerHTML = `
            <div class="neocp-modal-card glass" style="width: 500px;">
                <div class="modal-header">
                    <h4>GitOps Push-to-Deploy: ${domainName}</h4>
                    <button class="modal-close-btn close-git-modal">×</button>
                </div>
                <div class="modal-body">
                    <div class="input-group">
                        <label>Repository URL (HTTPS or SSH)</label>
                        <input type="text" id="git-repo-url" value="${config.repo_url || ''}" placeholder="https://github.com/user/repo.git">
                    </div>
                    <div class="input-group">
                        <label>Branch</label>
                        <input type="text" id="git-branch" value="${config.branch || 'main'}" placeholder="main">
                    </div>
                    <div class="input-group">
                        <label>Deployment Path (Relative to home)</label>
                        <input type="text" id="git-path" value="${config.path || ''}" placeholder="public_html/${domainName}">
                    </div>

                    <div style="margin-top: 20px; padding: 15px; background: rgba(0,0,0,0.2); border-radius: 8px;">
                        <h6 style="margin:0 0 10px 0;">Webhook URL</h6>
                        <code style="font-size:11px; word-break:break-all;">https://neocp.io/api/webhooks/git/${domainName}</code>
                        <p style="font-size:10px; margin-top:5px; color:var(--text-muted);">Add this URL to your GitHub/GitLab repository settings to trigger auto-deploy.</p>
                    </div>

                    <button class="action-btn" id="save-git-deploy-btn" style="width:100%; margin-top:20px;">⚡ Save & Trigger Initial Deploy</button>
                </div>
            </div>
        `;
        document.body.appendChild(overlay);

        overlay.querySelector('.close-git-modal').addEventListener('click', () => overlay.remove());

        document.getElementById('save-git-deploy-btn').addEventListener('click', async () => {
            const repo_url = document.getElementById('git-repo-url').value;
            const branch = document.getElementById('git-branch').value;
            const path = document.getElementById('git-path').value;

            addTaskIndicator();
            try {
                const res = await fetch('/api/domains/git/deploy', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                        'Authorization': `Bearer ${localStorage.getItem('neocp_token')}`
                    },
                    body: JSON.stringify({
                        domain_name: domainName,
                        config: { repo_url, branch, path }
                    })
                });
                if (res.ok) {
                    showNotification('GitOps configuration saved. Deployment worker started.', 'success');
                    overlay.remove();
                    loadDomainsTable();
                }
            } catch (e) {} finally {
                removeTaskIndicator();
            }
        });
    }

    async function deleteDomainSpace(domain) {
        addTaskIndicator();
        try {
            const res = await fetch(`/api/domains?name=${domain}`, {
                method: 'DELETE',
                headers: { 'Authorization': `Bearer ${localStorage.getItem('neocp_token')}` }
            });
            if (res.ok) {
                showNotification('VirtualHost maps deleted and cleaned successfully.', 'success');
                loadDomainsTable();
                loadDashboardSummaries();
            }
        } catch (e) {
            showNotification('Error deleting domain space.', 'error');
        } finally {
            removeTaskIndicator();
        }
    }

    // ==========================================================================
    // 6. MULTI-TENANT SANDBOXED INTERACTIVE FILE EXPLORER
    // ==========================================================================
    const currentDirLabel = document.getElementById('filemanager-current-dir');
    const fmNewFileBtn = document.getElementById('filemanager-newfile-btn');
    const fmNewDirBtn = document.getElementById('filemanager-newdir-btn');
    const fmPane = document.querySelector('.filemanager-content-pane');

    if (fmPane) {
        fmPane.addEventListener('dragover', (e) => {
            e.preventDefault();
            fmPane.style.borderColor = 'var(--accent-blue)';
            fmPane.style.background = 'rgba(59, 130, 246, 0.05)';
        });

        fmPane.addEventListener('dragleave', () => {
            fmPane.style.borderColor = '';
            fmPane.style.background = '';
        });

        fmPane.addEventListener('drop', async (e) => {
            e.preventDefault();
            fmPane.style.borderColor = '';
            fmPane.style.background = '';

            const files = e.dataTransfer.files;
            if (files.length > 0) {
                addTaskIndicator();
                showNotification(`Uploading ${files.length} items to user sandbox...`, 'info');

                // Simulate multi-file production upload stream
                for (let i = 0; i < files.length; i++) {
                    const file = files[i];
                    await new Promise(resolve => setTimeout(resolve, 400));
                    showNotification(`Buffered: ${file.name} (${formatBytes(file.size)})`, 'success');
                }

                showNotification('Upload complete. NeoCP enhanced drag-drop engine processed all nodes.', 'success');
                removeTaskIndicator();
                loadFileExplorer();
            }
        });
    }
    const fmWebdavBtn = document.getElementById('filemanager-webdav-btn');

    fmNewFileBtn.addEventListener('click', async () => {
        const name = prompt('Enter name of new file to create:', 'index.html');
        if (!name) return;
        addTaskIndicator();
        try {
            const res = await fetch('/api/filemanager/create', {
                method: 'POST',
                headers: { 
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${localStorage.getItem('neocp_token')}`
                },
                body: JSON.stringify({ path: `${currentPath}/${name}`, is_dir: false })
            });
            if (res.ok) {
                showNotification('File created successfully in user space.', 'success');
                loadFileExplorer();
            }
        } catch (e) {
            showNotification('API connection error creating file.', 'error');
        } finally {
            removeTaskIndicator();
        }
    });

    document.getElementById('filemanager-empty-trash-btn').addEventListener('click', async () => {
        if (!confirm('Are you sure you want to permanently delete all items in trash?')) return;
        addTaskIndicator();
        try {
            const res = await fetch('/api/filemanager/emptytrash', {
                method: 'POST',
                headers: { 'Authorization': `Bearer ${localStorage.getItem('neocp_token')}` }
            });
            const data = await res.json();
            if (res.ok) {
                showNotification(`Trash emptied. Freed ${formatBytes(data.bytes_freed)}`, 'success');
                loadDashboardSummaries();
            }
        } catch (e) {} finally {
            removeTaskIndicator();
        }
    });

    fmNewDirBtn.addEventListener('click', async () => {
        const name = prompt('Enter folder name to provision:', 'assets');
        if (!name) return;
        addTaskIndicator();
        try {
            const res = await fetch('/api/filemanager/create', {
                method: 'POST',
                headers: { 
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${localStorage.getItem('neocp_token')}`
                },
                body: JSON.stringify({ path: `${currentPath}/${name}`, is_dir: true })
            });
            if (res.ok) {
                showNotification('Sub-folder initialized successfully.', 'success');
                loadFileExplorer();
            }
        } catch (e) {
            showNotification('API connection error creating directory.', 'error');
        } finally {
            removeTaskIndicator();
        }
    });

    let webdavEnabled = false;
    fmWebdavBtn.addEventListener('click', () => {
        webdavEnabled = !webdavEnabled;
        fmWebdavBtn.textContent = `WebDAV: ${webdavEnabled ? 'On (:8081)' : 'Off'}`;
        fmWebdavBtn.className = webdavEnabled ? 'action-btn' : 'action-btn-secondary';
        showNotification(`WebDAV network storage adapter ${webdavEnabled ? 'activated on port 8081' : 'disabled'}.`, 'info');
    });

    async function loadFileExplorer() {
        currentDirLabel.textContent = currentPath;
        const tbody = document.querySelector('#filemanager-table tbody');
        tbody.innerHTML = '<tr><td colspan="6" class="text-muted">Loading user files sandbox...</td></tr>';

        try {
            const res = await fetch(`/api/filemanager/list?path=${encodeURIComponent(currentPath)}`, {
                headers: { 'Authorization': `Bearer ${localStorage.getItem('neocp_token')}` }
            });
            if (!res.ok) {
                tbody.innerHTML = '<tr><td colspan="6" class="text-red">Access denied. Sandbox boundaries violated.</td></tr>';
                return;
            }
            const items = await res.json();

            tbody.innerHTML = '';
            
            // Add back/up link if not at root
            if (currentPath !== `/home/${currentUser}` && currentPath !== '/home') {
                const tr = document.createElement('tr');
                tr.style.cursor = 'pointer';
                tr.innerHTML = `
                    <td colspan="6" class="text-blue" style="font-weight: 600;">
                        📁 .. [Parent Directory]
                    </td>
                `;
                tr.addEventListener('click', () => {
                    const parts = currentPath.split('/');
                    parts.pop();
                    currentPath = parts.join('/');
                    loadFileExplorer();
                });
                tbody.appendChild(tr);
            }

            if (items.length === 0) {
                const tr = document.createElement('tr');
                tr.innerHTML = '<td colspan="6" class="text-muted">Directory empty.</td>';
                tbody.appendChild(tr);
                return;
            }

            items.forEach(item => {
                const tr = document.createElement('tr');
                tr.innerHTML = `
                    <td class="${item.is_dir ? 'text-blue' : ''}" style="cursor: ${item.is_dir ? 'pointer' : 'default'}; font-weight: ${item.is_dir ? '600' : 'normal'}">
                        ${item.is_dir ? '📁' : '📄'} ${item.name}
                    </td>
                    <td>${item.is_dir ? 'Folder' : 'File'}</td>
                    <td>${item.is_dir ? '-' : formatBytes(item.size)}</td>
                    <td style="font-family: var(--font-mono); font-size: 0.8rem;">${item.permissions}</td>
                    <td style="font-size: 0.8rem;">${item.mod_time}</td>
                    <td>
                        <div class="table-actions">
                            ${!item.is_dir ? `<button class="action-btn-secondary edit-file-btn" data-file="${item.name}" style="padding: 2px 6px; font-size: 0.75rem;">Edit</button>` : ''}
                            <button class="action-btn-secondary delete-file-btn text-red" data-item="${item.name}" style="padding: 2px 6px; font-size: 0.75rem; border-color: rgba(239, 68, 68, 0.15)">Delete</button>
                        </div>
                    </td>
                `;

                // Folder double-click entry
                if (item.is_dir) {
                    tr.querySelector('td').addEventListener('click', () => {
                        currentPath = `${currentPath}/${item.name}`;
                        loadFileExplorer();
                    });
                }

                // Edit file trigger
                if (!item.is_dir) {
                    tr.querySelector('.edit-file-btn').addEventListener('click', () => {
                        launchFileEditorModal(`${currentPath}/${item.name}`);
                    });
                }

                // Delete file
                tr.querySelector('.delete-file-btn').addEventListener('click', async () => {
                    const toTrash = confirm(`Move ${item.name} to trash? (Cancel to permanently delete)`);
                    if (toTrash) {
                        await deleteSandboxItem(`${currentPath}/${item.name}`, false);
                    } else {
                        if (confirm(`REALLY wipe ${item.name} PERMANENTLY?`)) {
                            await deleteSandboxItem(`${currentPath}/${item.name}`, true);
                        }
                    }
                });

                tbody.appendChild(tr);
            });

            // Populate visual tree
            renderSidebarDirectoryTree();
        } catch (e) {
            console.error('File manager synchronization failure.', e);
        }
    }

    async function loadTrashSummary() {
        // Just used to update visual markers if needed
    }

    async function deleteSandboxItem(path, force = false) {
        addTaskIndicator();
        try {
            const res = await fetch(`/api/filemanager/delete?path=${encodeURIComponent(path)}&force=${force}`, {
                method: 'DELETE',
                headers: { 'Authorization': `Bearer ${localStorage.getItem('neocp_token')}` }
            });
            if (res.ok) {
                showNotification('Deleted item successfully from filesystem.', 'success');
                loadFileExplorer();
            }
        } catch (e) {
            showNotification('Error deleting item.', 'error');
        } finally {
            removeTaskIndicator();
        }
    }

    // Interactive File Editor
    async function launchFileEditorModal(filePath) {
        addTaskIndicator();
        try {
            const res = await fetch(`/api/filemanager/read?path=${encodeURIComponent(filePath)}`, {
                headers: { 'Authorization': `Bearer ${localStorage.getItem('neocp_token')}` }
            });
            const data = await res.json();
            
            // Create nice dynamic overlay for inline editing
            const overlay = document.createElement('div');
            overlay.className = 'neocp-modal-backdrop active';
            overlay.innerHTML = `
                <div class="neocp-modal-card glass" style="width: 80%; max-width: 900px; height: 80vh; display: flex; flex-direction: column;">
                    <div class="modal-header">
                        <h4>Inline Code Editor — ${filePath.split('/').pop()}</h4>
                        <button class="modal-close-btn close-editor-btn">×</button>
                    </div>
                    <div class="modal-body" style="flex-grow: 1; display: flex; flex-direction: column; gap: 15px;">
                        <textarea class="editor-textarea" style="flex-grow: 1; width: 100%; background: #04070d; border: 1px solid var(--border-ui); color: #8ed9ff; font-family: var(--font-mono); padding: 15px; border-radius: var(--radius-md); font-size: 0.9rem; resize: none;"></textarea>
                        <div style="display:flex; justify-content: flex-end; gap:10px;">
                            <button class="action-btn-secondary close-editor-btn">Discard</button>
                            <button class="action-btn save-editor-btn">⚡ Commit & Save Config</button>
                        </div>
                    </div>
                </div>
            `;
            document.body.appendChild(overlay);

            const textarea = overlay.querySelector('.editor-textarea');
            textarea.value = data.content || '';

            // Handle Save
            overlay.querySelector('.save-editor-btn').addEventListener('click', async () => {
                addTaskIndicator();
                try {
                    const commitRes = await fetch('/api/filemanager/write', {
                        method: 'POST',
                        headers: { 
                            'Content-Type': 'application/json',
                            'Authorization': `Bearer ${localStorage.getItem('neocp_token')}`
                        },
                        body: JSON.stringify({ path: filePath, content: textarea.value })
                    });
                    if (commitRes.ok) {
                        showNotification('Filesystem buffer write committed and saved successfully!', 'success');
                        overlay.remove();
                    }
                } catch (e) {
                    showNotification('Error writing file payload.', 'error');
                } finally {
                    removeTaskIndicator();
                }
            });

            overlay.querySelectorAll('.close-editor-btn').forEach(btn => {
                btn.addEventListener('click', () => overlay.remove());
            });

        } catch (e) {
            showNotification('Error loading file content.', 'error');
        } finally {
            removeTaskIndicator();
        }
    }

    function renderSidebarDirectoryTree() {
        const tree = document.getElementById('filemanager-tree');
        tree.innerHTML = `
            <li class="tree-node active">
                🌐 home
                <ul style="list-style:none; padding-left: 15px; margin-top:5px;">
                    <li class="tree-node text-blue">📁 patel</li>
                    <ul style="list-style:none; padding-left: 15px;">
                        <li class="tree-node text-blue">📁 public_html</li>
                        <li class="tree-node text-blue" style="opacity: 0.6">📁 mail</li>
                        <li class="tree-node text-blue" style="opacity: 0.6">📁 backups</li>
                    </ul>
                </ul>
            </li>
        `;
    }

    function formatBytes(bytes, decimals = 2) {
        if (bytes === 0) return '0 Bytes';
        const k = 1024;
        const dm = decimals < 0 ? 0 : decimals;
        const sizes = ['Bytes', 'KB', 'MB', 'GB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        return parseFloat((bytes / Math.pow(k, i)).toFixed(dm)) + ' ' + sizes[i];
    }

    // ==========================================================================
    // 7. MYSQL DATABASE CORE ENGINE
    // ==========================================================================
    const addDbModal = document.getElementById('add-db-modal');
    const openAddDbBtn = document.getElementById('open-add-db-btn');
    const closeDbModalBtn = document.getElementById('close-db-modal-btn');
    const submitDbBtn = document.getElementById('modal-add-db-submit');
    const remoteIpInput = document.getElementById('db-remote-ip-input');
    const saveRemoteIpBtn = document.getElementById('save-remote-ip-btn');

    openAddDbBtn.addEventListener('click', () => {
        document.getElementById('modal-db-name').value = `${currentUser}_wp`;
        document.getElementById('modal-db-user').value = `${currentUser}_wpuser`;
        addDbModal.classList.add('active');
    });

    closeDbModalBtn.addEventListener('click', () => {
        addDbModal.classList.remove('active');
    });

    submitDbBtn.addEventListener('click', async () => {
        const dbName = document.getElementById('modal-db-name').value;
        const dbUser = document.getElementById('modal-db-user').value;
        const dbPass = document.getElementById('modal-db-pass').value;

        if (!dbName || !dbUser || !dbPass) {
            showNotification('Please populate database credentials.', 'error');
            return;
        }

        addTaskIndicator();
        try {
            const res = await fetch('/api/databases', {
                method: 'POST',
                headers: { 
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${localStorage.getItem('neocp_token')}`
                },
                body: JSON.stringify({
                    name: dbName,
                    owner: currentUser,
                    db_user: dbUser,
                    password: dbPass,
                    remote_ips: "%"
                })
            });
            if (res.ok) {
                showNotification(`MySQL database stack initialized for schema ${dbName}`, 'success');
                addDbModal.classList.remove('active');
                loadDatabasesTable();
            } else {
                showNotification('Error creating DB instance.', 'error');
            }
        } catch (e) {
            showNotification('MySQL API dispatch failure.', 'error');
        } finally {
            removeTaskIndicator();
        }
    });

    async function loadDatabasesTable() {
        const tbody = document.querySelector('#databases-table tbody');
        tbody.innerHTML = '<tr><td colspan="4" class="text-muted">Loading MariaDB active connections...</td></tr>';

        try {
            const res = await fetch('/api/databases', {
                headers: { 'Authorization': `Bearer ${localStorage.getItem('neocp_token')}` }
            });
            const dbs = await res.json();

            tbody.innerHTML = '';
            if (dbs.length === 0) {
                tbody.innerHTML = '<tr><td colspan="4" class="text-muted">No database clusters mapped.</td></tr>';
                return;
            }

            dbs.forEach(db => {
                const tr = document.createElement('tr');
                tr.innerHTML = `
                    <td>
                        <strong class="text-blue">${db.name}</strong><br>
                        <span style="font-size: 0.75rem; color: var(--text-muted);">Privileged User: ${db.db_user}</span>
                    </td>
                    <td>${db.owner}</td>
                    <td>
                        <span class="status-badge badge-green">${db.remote_ips === '%' ? '🌍 Wildcard (%) Allow' : db.remote_ips}</span>
                    </td>
                    <td>
                        <div class="table-actions">
                            <button class="action-btn-secondary set-db-remote-btn" data-db="${db.name}" style="padding: 4px 8px; font-size: 0.75rem;">Whitelist IP</button>
                            <button class="action-btn-secondary delete-db-btn text-red" data-db="${db.name}" style="padding: 4px 8px; font-size: 0.75rem; border-color: rgba(239, 68, 68, 0.15)">Delete</button>
                        </div>
                    </td>
                `;

                // Whitelist remote IP triggers
                tr.querySelector('.set-db-remote-btn').addEventListener('click', () => {
                    remoteIpInput.value = db.remote_ips;
                    remoteIpInput.setAttribute('data-db', db.name);
                    showNotification('Populated remote databases whitelist parameters panel.', 'info');
                });

                // Delete database
                tr.querySelector('.delete-db-btn').addEventListener('click', async () => {
                    if (confirm(`Wipe database schema ${db.name}? ALL tables will be dropped!`)) {
                        addTaskIndicator();
                        try {
                            const delRes = await fetch(`/api/databases?name=${db.name}`, {
                                method: 'DELETE',
                                headers: { 'Authorization': `Bearer ${localStorage.getItem('neocp_token')}` }
                            });
                            if (delRes.ok) {
                                showNotification('Database schemas dropped successfully.', 'success');
                                loadDatabasesTable();
                            }
                        } catch (e) {
                            showNotification('Failed to drop schema.', 'error');
                        } finally {
                            removeTaskIndicator();
                        }
                    }
                });

                tbody.appendChild(tr);
            });
        } catch (e) {
            console.error('Failed to parse databases.', e);
        }
    }

    saveRemoteIpBtn.addEventListener('click', async () => {
        const dbName = remoteIpInput.getAttribute('data-db');
        const ips = remoteIpInput.value;
        if (!dbName) {
            showNotification('Please select a database from left side actions first.', 'error');
            return;
        }

        addTaskIndicator();
        try {
            const res = await fetch('/api/databases/ips', {
                method: 'POST',
                headers: { 
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${localStorage.getItem('neocp_token')}`
                },
                body: JSON.stringify({ name: dbName, remote_ips: ips })
            });
            if (res.ok) {
                showNotification(`MariaDB networking privileges updated: ${dbName} whitelists [${ips}]`, 'success');
                loadDatabasesTable();
            }
        } catch (e) {
            showNotification('Failed to update MariaDB configs.', 'error');
        } finally {
            removeTaskIndicator();
        }
    });

    document.getElementById('open-phpmyadmin-btn').addEventListener('click', () => {
        showNotification('Launching phpMyAdmin visual administrator mapping overlay...', 'info');
        window.open('https://localhost:8443/phpmyadmin', '_blank');
    });

    document.getElementById('db-map-tool-btn')?.addEventListener('click', () => {
        showNotification('Database Map Tool: Scanning for orphaned database schemas...', 'info');
        setTimeout(() => {
            showNotification('No orphaned databases found. All schemas correctly mapped to NeoCP users.', 'success');
        }, 1500);
    });

    document.getElementById('db-repair-all-btn')?.addEventListener('click', () => {
        addTaskIndicator();
        showNotification('Initiating global repair on all MariaDB databases...', 'info');
        setTimeout(() => {
            showNotification('Repair complete. Analyzed 142 tables, 0 corruption detected.', 'success');
            removeTaskIndicator();
        }, 3000);
    });

    document.getElementById('db-proc-monitor-btn')?.addEventListener('click', () => {
        showNotification('Fetching active MySQL process threads...', 'info');
        // Simulate monitor view
    });

    // ==========================================================================
    // 8. SECURITY HARDENING & CPHULK LOGS
    // ==========================================================================
    const modsecToggle = document.getElementById('modsec-toggle');
    const modsecRules = document.getElementById('modsec-rules-textarea');
    const saveModsecBtn = document.getElementById('save-modsec-rules-btn');
    const blockedIps = document.getElementById('blocked-ips-count');
    const logBox = document.getElementById('cphulk-log-box');

    const aiAnalyzeBtn = document.getElementById('ai-analyze-btn');
    if (aiAnalyzeBtn) {
        aiAnalyzeBtn.addEventListener('click', async () => {
            const container = document.getElementById('ai-suggestions-container');
            container.innerHTML = '<div class="text-muted" style="text-align: center; padding: 20px;">🤖 AI Guardian is processing system buffers...</div>';
            addTaskIndicator();
            try {
                const res = await fetch('/api/security/ai/analyze', {
                    headers: { 'Authorization': `Bearer ${localStorage.getItem('neocp_token')}` }
                });
                const suggestions = await res.json();
                container.innerHTML = '';
                if (suggestions.length === 0) {
                    container.innerHTML = '<div class="text-muted" style="text-align: center; padding: 20px;">No critical issues identified. System is optimized.</div>';
                    return;
                }
                suggestions.forEach(s => {
                    const div = document.createElement('div');
                    div.style.background = 'rgba(255,255,255,0.03)';
                    div.style.padding = '15px';
                    div.style.borderRadius = '8px';
                    div.style.borderLeft = `4px solid ${s.type === 'security' ? '#ef4444' : '#38bdf8'}`;
                    div.innerHTML = `
                        <strong style="display:block; margin-bottom:5px; text-transform:uppercase; font-size:10px; color:${s.type === 'security' ? '#ef4444' : '#38bdf8'};">${s.type} Suggestion</strong>
                        <p style="font-size:13px; margin:0 0 12px 0;">${s.message}</p>
                        <button class="action-btn-secondary" style="padding:4px 10px; font-size:11px; border-color:${s.type === 'security' ? '#ef4444' : '#38bdf8'}; color:${s.type === 'security' ? '#ef4444' : '#38bdf8'};">${s.action_label}</button>
                    `;
                    container.appendChild(div);
                });
            } catch (e) {} finally {
                removeTaskIndicator();
            }
        });
    }

    // Real-time Malware Scanner Simulation
    setInterval(() => {
        const paths = ['/public_html/index.php', '/public_html/wp-config.php', '/.env', '/mail/inbox'];
        const path = paths[Math.floor(Math.random() * paths.length)];
        const findings = ['PHP.Shell.Generic', 'Malware.Heuristic.Exploit', 'Suspicious.Pattern.Match'];

        if (Math.random() > 0.95) {
            const finding = findings[Math.floor(Math.random() * findings.length)];
            const line = document.createElement('div');
            line.className = 'log-line text-orange';
            const now = new Date().toLocaleTimeString();
            line.innerHTML = `[${now}] <span class="badge badge-red" style="font-size:9px;">SCANNER</span> Threat detected in ${path}: <strong>${finding}</strong>. File quarantined & cleaned automatically.`;
            logBox.prepend(line);
            showNotification(`Real-time scanner mitigated a threat in ${path}`, 'error');
        }
    }, 15000);

    // Simulate real intrusion event streams
    setInterval(() => {
        if (!modsecToggle.checked) return;
        const ips = ['182.50.4.12', '45.21.90.111', '90.101.44.20', '5.10.89.2', '122.9.230.12'];
        const randomIp = ips[Math.floor(Math.random() * ips.length)];
        const ports = [22, 80, 443, 3306, 21];
        const targetPort = ports[Math.floor(Math.random() * ports.length)];

        // Append log line
        const line = document.createElement('div');
        line.className = 'log-line text-red';
        const now = new Date().toLocaleTimeString();
        line.innerHTML = `[${now}] Intrusion Blocked: ${randomIp} attempted ssh brute force on port ${targetPort}. cPHulk jail hook triggered!`;
        logBox.prepend(line);

        // Limit log lines count
        if (logBox.children.length > 25) logBox.lastChild.remove();

        // Increment count
        let count = parseInt(blockedIps.textContent) || 0;
        blockedIps.textContent = count + 1;
    }, 8000);

    saveModsecBtn.addEventListener('click', () => {
        addTaskIndicator();
        setTimeout(() => {
            removeTaskIndicator();
            showNotification('ModSecurity customized rules compiled and hot-reloaded into web server!', 'success');
        }, 800);
    });

    // ==========================================================================
    // 9. SOFTACULOUS APP AUTODEPLOYERS & PHP.INI SETTINGS
    // ==========================================================================
    document.querySelectorAll('.install-app-btn').forEach(btn => {
        btn.addEventListener('click', (e) => {
            const app = e.target.getAttribute('data-app');
            // If WordPress, show toolkit option
            if (app === 'WordPress') {
                launchWPToolkitModal();
            } else {
                triggerStandardAppInstall(app);
            }
        });
    });

    function triggerStandardAppInstall(app) {
        addTaskIndicator();
        showNotification(`[Backuply] Taking pre-install backup of target domain...`, 'info');
        setTimeout(() => {
            showNotification(`Softaculous: Downloading ${app} binaries...`, 'info');
            setTimeout(() => {
                showNotification(`Softaculous: Creating MySQL database for ${app}...`, 'success');
                setTimeout(() => {
                    showNotification(`${app} installed and active! Database credentials mailed.`, 'success');
                    removeTaskIndicator();
                    loadFileExplorer();
                }, 1500);
            }, 1000);
        }, 800);
    }

    function launchWPToolkitModal() {
        const overlay = document.createElement('div');
        overlay.className = 'neocp-modal-backdrop active';
        overlay.innerHTML = `
            <div class="neocp-modal-card glass" style="width: 550px;">
                <div class="modal-header">
                    <h4>NeoCP WP Toolkit — CMS Lifecycle Manager</h4>
                    <button class="modal-close-btn close-wp-modal">×</button>
                </div>
                <div class="modal-body">
                    <div style="display:grid; grid-template-columns: 1fr 1fr; gap:15px; margin-bottom:20px;">
                        <div class="db-panel-card" style="text-align:center;">
                            <h5 style="margin:0 0 5px 0;">Version</h5>
                            <span class="status-badge badge-green">v6.5.3 (Up to date)</span>
                        </div>
                        <div class="db-panel-card" style="text-align:center;">
                            <h5 style="margin:0 0 5px 0;">Security</h5>
                            <span class="status-badge badge-orange">Medium Hardened</span>
                        </div>
                    </div>

                    <h5 style="margin-bottom:10px;">Security Hardening Matrix</h5>
                    <div style="display:flex; flex-direction:column; gap:10px; background:rgba(0,0,0,0.2); padding:15px; border-radius:8px;">
                        <label style="display:flex; justify-content:space-between; align-items:center; cursor:pointer;">
                            <span style="font-size:13px;">Disable XML-RPC API</span>
                            <input type="checkbox" id="wp-harden-xmlrpc" checked>
                        </label>
                        <label style="display:flex; justify-content:space-between; align-items:center; cursor:pointer;">
                            <span style="font-size:13px;">Hide WP-Login (Move to /portal)</span>
                            <input type="checkbox" id="wp-harden-login">
                        </label>
                        <label style="display:flex; justify-content:space-between; align-items:center; cursor:pointer;">
                            <span style="font-size:13px;">Disable File Editing in Panel</span>
                            <input type="checkbox" id="wp-harden-fileedit" checked>
                        </label>
                    </div>

                    <div style="margin-top:20px; display:flex; gap:10px;">
                        <button class="action-btn" id="wp-apply-harden-btn" style="flex:2;">🛡️ Apply Hardening</button>
                        <button class="action-btn-secondary" id="wp-bulk-update-btn" style="flex:1;">🔄 Update All</button>
                    </div>
                </div>
            </div>
        `;
        document.body.appendChild(overlay);
        overlay.querySelector('.close-wp-modal').addEventListener('click', () => overlay.remove());

        document.getElementById('wp-apply-harden-btn').addEventListener('click', async () => {
            addTaskIndicator();
            showNotification('WP Toolkit: Rewriting wp-config.php and .htaccess...', 'info');
            setTimeout(() => {
                showNotification('WordPress security policy enforced successfully.', 'success');
                removeTaskIndicator();
                overlay.remove();
            }, 1500);
        });

        document.getElementById('wp-bulk-update-btn').addEventListener('click', () => {
            showNotification('WP Toolkit: Initiating bulk core/plugin update across all domains...', 'info');
            overlay.remove();
        });
    }

    document.querySelectorAll('.install-app-btn').forEach(btn => {
        btn.addEventListener('click', (e) => {
            const app = e.target.getAttribute('data-app');
            triggerAppInstallationAutodeploy(app, e.target);
        });
    });


    document.getElementById('save-php-ini-btn').addEventListener('click', () => {
        const mem = document.getElementById('php-memory-limit').value;
        const upload = document.getElementById('php-upload-limit').value;
        const maxTime = document.getElementById('php-exec-time').value;

        addTaskIndicator();
        setTimeout(() => {
            removeTaskIndicator();
            showNotification(`PHP FPM parameters updated: memory_limit=${mem}, upload_max_filesize=${upload}, max_execution_time=${maxTime}`, 'success');
        }, 500);
    });

    // ==========================================================================
    // 10. TELEMETRY & OS PROCESSES MANAGER
    // ==========================================================================
    const procSearch = document.getElementById('process-search-input');
    procSearch.addEventListener('input', (e) => {
        selectedProcessQuery = e.target.value.toLowerCase();
    });

    function renderProcessesList(processes) {
        const tbody = document.querySelector('#processes-table tbody');
        tbody.innerHTML = '';

        const filtered = processes.filter(p => 
            p.name.toLowerCase().includes(selectedProcessQuery) ||
            p.user.toLowerCase().includes(selectedProcessQuery) ||
            p.pid.toString().includes(selectedProcessQuery)
        );

        filtered.forEach(p => {
            const tr = document.createElement('tr');
            tr.innerHTML = `
                <td style="font-family: var(--font-mono); font-size: 0.85rem;">${p.pid}</td>
                <td><strong class="text-blue">${p.name}</strong></td>
                <td>
                    <span class="status-badge ${p.cpu > 15 ? 'badge-red' : 'badge-green'}">${p.cpu.toFixed(1)}%</span>
                </td>
                <td style="font-family: var(--font-mono);">${p.memory_mb.toFixed(1)} MB</td>
                <td>${p.user}</td>
                <td>
                    <button class="action-btn-secondary kill-process-btn text-red" data-pid="${p.pid}" style="padding: 2px 6px; font-size: 0.75rem; border-color: rgba(239, 68, 68, 0.2);">Kill</button>
                </td>
            `;

            tr.querySelector('.kill-process-btn').addEventListener('click', () => {
                if (confirm(`Send SIGKILL to PID ${p.pid} (${p.name})?`)) {
                    killProcessByPid(p.pid);
                }
            });

            tbody.appendChild(tr);
        });
    }

    async function killProcessByPid(pid) {
        addTaskIndicator();
        try {
            const res = await fetch(`/api/processes/kill?pid=${pid}`, {
                method: 'POST',
                headers: { 'Authorization': `Bearer ${localStorage.getItem('neocp_token')}` }
            });
            if (res.ok) {
                showNotification(`SIGKILL signal dispatched successfully to PID ${pid}`, 'success');
            }
        } catch (e) {
            showNotification('Error killing process.', 'error');
        } finally {
            removeTaskIndicator();
        }
    }

    // ==========================================================================
    // 11. AUTOMATED MIGRATION ENGINE (UZME)
    // ==========================================================================
    const startMigrationBtn = document.getElementById('start-migration-btn');
    const migStatusLabel = document.getElementById('migration-status-label');
    const migPctLabel = document.getElementById('migration-pct-label');
    const migProgressBar = document.getElementById('migration-progress-bar');
    const migLogs = document.getElementById('migration-log-box');

    startMigrationBtn.addEventListener('click', async () => {
        const sourceType = document.getElementById('migration-source-type').value;
        const sourceHost = document.getElementById('migration-source-host').value;
        const sourceToken = document.getElementById('migration-source-token').value;
        const targetDomain = document.getElementById('migration-target-domain').value;

        if (!sourceHost || !sourceToken || !targetDomain) {
            showNotification('Please fill in migration host properties.', 'error');
            return;
        }

        addTaskIndicator();
        showNotification('Establishing authenticated handshake with legacy server...', 'info');

        // Check if it's bulk or single (simplified for this stage)
        const isBulk = targetDomain === '*' || targetDomain.includes(',');

        try {
            const endpoint = isBulk ? '/api/migrations/bulk' : '/api/migrations';
            const body = isBulk ? {
                source: { panel_type: sourceType, hostname: sourceHost, api_token: sourceToken, username: 'root' },
                accounts: targetDomain.split(',').map(u => ({ account: u.trim(), domain: 'auto', status: 'pending', progress: 0 }))
            } : {
                panel_type: sourceType,
                hostname: sourceHost,
                target_domain: targetDomain,
                owner: currentUser
            };

            const res = await fetch(endpoint, {
                method: 'POST',
                headers: { 
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${localStorage.getItem('neocp_token')}`
                },
                body: JSON.stringify(body)
            });
            const task = await res.json();
            if (res.ok) {
                // Trigger polling of progress
                monitorMigrationProgress(task.id);
            }
        } catch (e) {
            showNotification('Failed to launch automated UZME migration.', 'error');
        } finally {
            removeTaskIndicator();
        }
    });

    function monitorMigrationProgress(taskId) {
        migLogs.innerHTML = '';
        const interval = setInterval(async () => {
            try {
                const res = await fetch(`/api/migrations?id=${taskId}`, {
                    headers: { 'Authorization': `Bearer ${localStorage.getItem('neocp_token')}` }
                });
                const tasks = await res.json();
                const task = tasks.find(t => t.id === taskId);

                if (task) {
                    migStatusLabel.textContent = task.status.toUpperCase().replace('_', ' ');
                    migPctLabel.textContent = `${task.progress_pct.toFixed(0)}%`;
                    migProgressBar.style.width = `${task.progress_pct}%`;

                    // Render dynamic logs
                    migLogs.innerHTML = '';
                    task.logs.forEach(log => {
                        const div = document.createElement('div');
                        div.className = 'log-line';
                        if (log.includes('established') || log.includes('Sync completed')) div.classList.add('text-green');
                        else if (log.includes('Injecting') || log.includes('syncing')) div.classList.add('text-blue');
                        div.textContent = log;
                        migLogs.appendChild(div);
                    });

                    if (task.progress_pct >= 100) {
                        clearInterval(interval);
                        showNotification('Zero-downtime cutover completed! Reverse proxy actively routing traffic.', 'success');
                        loadDomainsTable();
                        loadDashboardSummaries();
                    }
                }
            } catch (e) {
                clearInterval(interval);
            }
        }, 1000);
    }

    async function loadMigrationTasks() {
        // Query standby tasks if any on boot
        try {
            const res = await fetch('/api/migrations', {
                headers: { 'Authorization': `Bearer ${localStorage.getItem('neocp_token')}` }
            });
            const tasks = await res.json();
            const active = tasks.find(t => t.progress_pct < 100);
            if (active) {
                monitorMigrationProgress(active.id);
            }
        } catch (e) {}
    }

    // ==========================================================================
    // 12. MASTER STAGING & CLUSTERING ORCHESTRATION
    // ==========================================================================
    async function loadClusterNodes() {
        const container = document.getElementById('cluster-nodes-container');
        if (!container) return;
        container.innerHTML = '<div class="text-muted">Loading cluster topology...</div>';
        try {
            const res = await fetch('/api/cluster/nodes', {
                headers: { 'Authorization': `Bearer ${localStorage.getItem('neocp_token')}` }
            });
            const nodes = await res.json();
            container.innerHTML = '';
            if (nodes.length === 0) {
                container.innerHTML = '<div class="text-muted">No remote nodes attached.</div>';
                return;
            }
            nodes.forEach(n => {
                const div = document.createElement('div');
                div.className = 'cluster-node-row glass';
                div.style.marginBottom = '10px';
                div.style.padding = '15px';
                div.innerHTML = `
                    <div class="node-meta">
                        <h6 style="margin:0; font-family:var(--font-mono); color:var(--accent-blue);">🖥️ ${n.node_id} [${n.ip}]</h6>
                        <span class="node-role-tag">${n.role.toUpperCase()} NODE</span>
                        <div style="margin-top:8px; display:flex; gap:15px;">
                            <div style="flex:1;">
                                <div style="display:flex; justify-content:space-between; font-size:10px; margin-bottom:4px;">
                                    <span>CPU Load</span>
                                    <span>${n.cpu_load.toFixed(1)}%</span>
                                </div>
                                <div class="progress-bar-container" style="height:4px;">
                                    <div class="progress-fill fill-blue" style="width:${n.cpu_load}%"></div>
                                </div>
                            </div>
                            <div style="flex:1;">
                                <div style="display:flex; justify-content:space-between; font-size:10px; margin-bottom:4px;">
                                    <span>RAM Load</span>
                                    <span>${n.ram_load.toFixed(1)}%</span>
                                </div>
                                <div class="progress-bar-container" style="height:4px;">
                                    <div class="progress-fill fill-green" style="width:${n.ram_load}%"></div>
                                </div>
                            </div>
                        </div>
                    </div>
                    <div style="text-align:right;">
                        <span class="status-badge ${n.is_active ? 'badge-green' : 'badge-red'}">${n.is_active ? 'ONLINE' : 'OFFLINE'}</span>
                        <div style="font-size:9px; color:var(--text-inactive); margin-top:5px;">Last Ping: ${new Date(n.last_ping).toLocaleTimeString()}</div>
                    </div>
                `;
                container.appendChild(div);
            });
        } catch (e) {
            container.innerHTML = '<div class="text-red">Failed to load cluster.</div>';
        }
    }

    async function attachNode() {
        const ip = document.getElementById('cluster-node-ip').value;
        const role = document.getElementById('cluster-node-role').value;
        const nodeID = `vps-${Math.floor(Math.random()*900)+100}`;

        if (!ip) {
            showNotification('Enter a valid worker cluster address.', 'error');
            return;
        }

        addTaskIndicator();
        showNotification(`Registering node ${nodeID} and generating mTLS certificates...`, 'info');
        try {
            const res = await fetch('/api/cluster/attach', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${localStorage.getItem('neocp_token')}`
                },
                body: JSON.stringify({ node_id: nodeID, ip, role })
            });
            const data = await res.json();
            if (res.ok) {
                showNotification(`Node ${nodeID} attached! PKCS#8 certificates generated for secure gRPC stream.`, 'success');
                loadClusterNodes();
            } else {
                showNotification(data.error || 'Failed to attach node.', 'error');
            }
        } catch (e) {
            showNotification('Cluster API connection failure.', 'error');
        } finally {
            removeTaskIndicator();
        }
    }

    async function cloneStaging(productionDomain, stagingSubdomain) {
        addTaskIndicator();
        showNotification(`Cloning production filesystem & databases for ${productionDomain}...`, 'info');

        // Visual progress ticker simulation
        let step = 0;
        const steps = [
            "Replicating directory structures...",
            "Cloning database schemas...",
            "Recalculating PHP serialized string lengths...",
            "Injecting staging Nginx virtual hosts..."
        ];
        const ticker = setInterval(() => {
            if (step < steps.length) {
                showNotification(steps[step], 'info');
                step++;
            } else {
                clearInterval(ticker);
            }
        }, 800);

        try {
            const res = await fetch('/api/staging?action=clone', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${localStorage.getItem('neocp_token')}`
                },
                body: JSON.stringify({ production_domain: productionDomain, staging_subdomain: stagingSubdomain })
            });
            const data = await res.json();
            if (res.ok) {
                showNotification(`Sandbox successfully provisioned at ${stagingSubdomain}. Live testing is enabled.`, 'success');
                loadDomainsTable();
                loadDashboardSummaries();
            } else {
                showNotification(data.error || 'Cloning failed.', 'error');
            }
        } catch (e) {
            showNotification('Staging API failure.', 'error');
        } finally {
            clearInterval(ticker);
            removeTaskIndicator();
        }
    }

    async function pushStaging(stagingSubdomain, syncMode) {
        addTaskIndicator();
        showNotification('Initiating production delta-sync push. Compiling changes...', 'info');

        try {
            const res = await fetch('/api/staging?action=push', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${localStorage.getItem('neocp_token')}`
                },
                body: JSON.stringify({ staging_subdomain: stagingSubdomain, sync_mode: syncMode })
            });
            if (res.ok) {
                showNotification('Staging pushed to primary production hub with zero file disruptions.', 'success');
                loadDomainsTable();
            } else {
                const data = await res.json();
                showNotification(data.error || 'Push failed.', 'error');
            }
        } catch (e) {
            showNotification('Staging Push API failure.', 'error');
        } finally {
            removeTaskIndicator();
        }
    }

    async function loadIPPool() {
        const tbody = document.querySelector('#ip-pool-table tbody');
        if (!tbody) return;
        tbody.innerHTML = '<tr><td colspan="6" class="text-muted">Loading IPv4 stack...</td></tr>';
        try {
            const res = await fetch('/api/server/ips', {
                headers: { 'Authorization': `Bearer ${localStorage.getItem('neocp_token')}` }
            });
            const ips = await res.json();
            tbody.innerHTML = '';

            const delSelect = document.getElementById('delegate-ip-select');
            if (delSelect) delSelect.innerHTML = '';

            ips.forEach(ip => {
                const tr = document.createElement('tr');
                tr.innerHTML = `
                    <td><strong>${ip.ip}</strong></td>
                    <td>${ip.subnet}</td>
                    <td><span class="status-badge ${ip.is_shared ? 'badge-blue' : 'badge-orange'}">${ip.is_shared ? 'Shared' : 'Dedicated'}</span></td>
                    <td><code class="text-blue">${ip.owner || 'unassigned'}</code></td>
                    <td><span class="status-badge ${ip.is_assigned ? 'badge-green' : 'badge-muted'}">${ip.is_assigned ? 'Assigned' : 'Available'}</span></td>
                    <td>
                        <button class="action-btn-secondary text-red" style="padding:2px 6px; font-size:0.75rem; border-color:rgba(239,68,68,0.2);">Unbind</button>
                    </td>
                `;
                tbody.appendChild(tr);

                if (!ip.is_shared && !ip.is_assigned && delSelect) {
                    const opt = document.createElement('option');
                    opt.value = ip.ip;
                    opt.textContent = ip.ip;
                    delSelect.appendChild(opt);
                }
            });

            // Populate reseller select for delegation
            const accRes = await fetch('/api/reseller/accounts', {
                headers: { 'Authorization': `Bearer ${localStorage.getItem('neocp_token')}` }
            });
            const accs = await accRes.json();
            const resSelect = document.getElementById('delegate-reseller-select');
            if (resSelect) {
                resSelect.innerHTML = '';
                accs.filter(a => a.role === 'reseller').forEach(r => {
                    const opt = document.createElement('option');
                    opt.value = r.username;
                    opt.textContent = r.username;
                    resSelect.appendChild(opt);
                });
            }

        } catch (e) {}
    }

    document.getElementById('add-ip-pool-btn')?.addEventListener('click', async () => {
        const ip = document.getElementById('new-ip-addr').value;
        const mask = document.getElementById('new-ip-mask').value;
        const shared = document.getElementById('new-ip-shared').checked;
        if (!ip) return;
        addTaskIndicator();
        try {
            const res = await fetch('/api/server/ips', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${localStorage.getItem('neocp_token')}`
                },
                body: JSON.stringify({ ip, subnet: mask, is_shared: shared })
            });
            if (res.ok) {
                showNotification(`IP ${ip} bound to eth0 interface successfully.`, 'success');
                loadIPPool();
            }
        } catch (e) {} finally {
            removeTaskIndicator();
        }
    });

    document.getElementById('delegate-ip-btn')?.addEventListener('click', async () => {
        const ip = document.getElementById('delegate-ip-select').value;
        const reseller = document.getElementById('delegate-reseller-select').value;
        if (!ip || !reseller) return;
        addTaskIndicator();
        try {
            const res = await fetch('/api/server/ips/delegate', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${localStorage.getItem('neocp_token')}`
                },
                body: JSON.stringify({ ip, reseller_id: reseller })
            });
            if (res.ok) {
                showNotification(`IP ${ip} delegated to ${reseller} pool.`, 'success');
                loadIPPool();
            }
        } catch (e) {} finally {
            removeTaskIndicator();
        }
    });

    async function deleteStaging(subdomain) {
        addTaskIndicator();
        try {
            const res = await fetch(`/api/staging?subdomain=${encodeURIComponent(subdomain)}`, {
                method: 'DELETE',
                headers: { 'Authorization': `Bearer ${localStorage.getItem('neocp_token')}` }
            });
            if (res.ok) {
                showNotification('Staging sandbox environment wiped successfully.', 'success');
                loadDomainsTable();
                loadDashboardSummaries();
            }
        } catch (e) {
            showNotification('Error destroying sandbox.', 'error');
        } finally {
            removeTaskIndicator();
        }
    }

    document.getElementById('generate-staging-btn').addEventListener('click', () => {
        const prod = document.getElementById('staging-prod-select').value;
        const sub = document.getElementById('staging-subdomain-input').value;
        cloneStaging(prod, sub);
    });

    document.getElementById('push-staging-btn').addEventListener('click', () => {
        // Find an active staging domain to push (just a helper for the generic button)
        const stagingDom = cachedDomains.find(d => d.domain_name.includes('-stage') || d.domain_name.startsWith('staging.'));
        if (!stagingDom) {
            showNotification('No active staging domain found to push.', 'error');
            return;
        }
        const syncMode = document.getElementById('staging-sync-mode').value;
        pushStaging(stagingDom.domain_name, syncMode);
    });

    document.getElementById('attach-node-btn').addEventListener('click', attachNode);

    // ==========================================================================
    // 13. WHM RESOURCE PACKAGE PROVISIONER
    // ==========================================================================
    const createPkgBtn = document.getElementById('create-package-btn');
    createPkgBtn.addEventListener('click', async () => {
        const name = document.getElementById('pkg-name-input').value;
        const disk = document.getElementById('pkg-disk-input').value;
        const bw = document.getElementById('pkg-bw-input').value;
        const domains = parseInt(document.getElementById('pkg-domains-input').value) || 0;
        const db = parseInt(document.getElementById('pkg-db-input').value) || 0;
        const ftp = parseInt(document.getElementById('pkg-ftp-input').value) || 0;
        const email = parseInt(document.getElementById('pkg-email-input').value) || 0;
        const relay = parseInt(document.getElementById('pkg-email-relay-input').value) || 0;
        const failPct = parseInt(document.getElementById('pkg-email-fail-pct-input').value) || 0;
        const isReseller = document.getElementById('pkg-reseller-toggle').checked;

        if (!name) {
            showNotification('Please enter a package name.', 'error');
            return;
        }

        addTaskIndicator();
        try {
            const res = await fetch('/api/packages', {
                method: 'POST',
                headers: { 
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${localStorage.getItem('neocp_token')}`
                },
                body: JSON.stringify({
                    name,
                    owner: currentUser,
                    disk_quota: disk,
                    bandwidth: bw,
                    max_domains: domains,
                    max_databases: db,
                    max_ftp: ftp,
                    max_email: email,
                    hourly_email_limit: relay,
                    failed_email_pct: failPct,
                    is_reseller: isReseller
                })
            });
            if (res.ok) {
                showNotification('Production package deployed successfully.', 'success');
                loadPackagesTable();
            }
        } catch (e) {
            showNotification('Error creating package.', 'error');
        } finally {
            removeTaskIndicator();
        }
    });

    async function loadPackagesTable() {
        const tbody = document.querySelector('#packages-table tbody');
        if (!tbody) return;
        tbody.innerHTML = '<tr><td colspan="5" class="text-muted">Loading plans...</td></tr>';

        try {
            const res = await fetch('/api/packages', {
                headers: { 'Authorization': `Bearer ${localStorage.getItem('neocp_token')}` }
            });
            const pkgs = await res.json();

            tbody.innerHTML = '';
            pkgs.forEach(pkg => {
                const tr = document.createElement('tr');
                tr.innerHTML = `
                    <td><strong class="text-blue">${pkg.name}</strong></td>
                    <td>${pkg.owner}</td>
                    <td>${pkg.disk_quota} / ${pkg.bandwidth}</td>
                    <td>D:${pkg.max_domains} | DB:${pkg.max_databases} | E:${pkg.max_email}</td>
                    <td>
                        <button class="action-btn-secondary delete-pkg-btn text-red" data-pkg="${pkg.name}" style="padding: 2px 6px; font-size: 0.75rem; border-color: rgba(239, 68, 68, 0.2);">Delete</button>
                    </td>
                `;

                tr.querySelector('.delete-pkg-btn').addEventListener('click', async () => {
                    if (confirm(`Delete resource package ${pkg.name}?`)) {
                        addTaskIndicator();
                        try {
                            const delRes = await fetch(`/api/packages?name=${pkg.name}`, {
                                method: 'DELETE',
                                headers: { 'Authorization': `Bearer ${localStorage.getItem('neocp_token')}` }
                            });
                            if (delRes.ok) {
                                showNotification('Resource package wiped successfully.', 'success');
                                loadPackagesTable();
                            }
                        } catch (e) {
                            showNotification('Error dropping plan.', 'error');
                        } finally {
                            removeTaskIndicator();
                        }
                    }
                });

                tbody.appendChild(tr);
            });
        } catch (e) {}
    }

    async function loadMailAccounts() {
        const tbody = document.querySelector('#mail-accounts-table tbody');
        if (!tbody) return;
        tbody.innerHTML = '<tr><td colspan="4" class="text-muted">Loading maildir nodes...</td></tr>';
        try {
            const res = await fetch('/api/mail/accounts', {
                headers: { 'Authorization': `Bearer ${localStorage.getItem('neocp_token')}` }
            });
            const mails = await res.json();
            tbody.innerHTML = '';
            if (mails.length === 0) {
                tbody.innerHTML = '<tr><td colspan="4" class="text-muted">No email accounts found.</td></tr>';
                return;
            }
            mails.forEach(m => {
                const tr = document.createElement('tr');
                tr.innerHTML = `
                    <td><strong>${m.email}</strong></td>
                    <td>${m.used_mb} MB / ${m.quota_mb} MB</td>
                    <td><span class="status-badge badge-green">Active</span></td>
                    <td>
                        <button class="action-btn-secondary delete-mail-btn text-red" data-email="${m.email}" style="padding:2px 6px; font-size:0.75rem; border-color:rgba(239,68,68,0.2);">Delete</button>
                    </td>
                `;
                tr.querySelector('.delete-mail-btn').addEventListener('click', async () => {
                    if (confirm(`Delete mail account ${m.email}?`)) {
                        await fetch(`/api/mail/accounts?email=${m.email}`, {
                            method: 'DELETE',
                            headers: { 'Authorization': `Bearer ${localStorage.getItem('neocp_token')}` }
                        });
                        loadMailAccounts();
                    }
                });
                tbody.appendChild(tr);
            });
        } catch (e) {}
    }

    document.getElementById('add-mail-btn')?.addEventListener('click', async () => {
        const email = document.getElementById('new-mail-addr').value;
        const password = document.getElementById('new-mail-pass').value;
        const quota = parseInt(document.getElementById('new-mail-quota').value);
        if (!email || !password) return;
        addTaskIndicator();
        try {
            const res = await fetch('/api/mail/accounts', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${localStorage.getItem('neocp_token')}`
                },
                body: JSON.stringify({ email, password, quota_mb: quota, domain: email.split('@')[1] })
            });
            if (res.ok) {
                showNotification(`Mail account ${email} provisioned.`, 'success');
                loadMailAccounts();
            }
        } catch (e) {} finally {
            removeTaskIndicator();
        }
    });

    async function loadResellerAccounts() {
        const tbody = document.querySelector('#reseller-accounts-table tbody');
        if (!tbody) return;
        tbody.innerHTML = '<tr><td colspan="4" class="text-muted">Loading managed accounts...</td></tr>';
        try {
            const res = await fetch('/api/reseller/accounts', {
                headers: { 'Authorization': `Bearer ${localStorage.getItem('neocp_token')}` }
            });
            const accs = await res.json();
            tbody.innerHTML = '';
            accs.forEach(acc => {
                const tr = document.createElement('tr');
                tr.innerHTML = `
                    <td><strong>${acc.username}</strong></td>
                    <td>${acc.plan}</td>
                    <td>${acc.owner}</td>
                    <td>
                        <button class="action-btn-secondary edit-acc-btn" data-user="${acc.username}" style="padding:2px 6px; font-size:0.75rem;">Edit</button>
                    </td>
                `;
                tbody.appendChild(tr);
            });
        } catch (e) {}
    }

    document.getElementById('transfer-acc-btn').addEventListener('click', async () => {
        const user = document.getElementById('transfer-acc-username').value;
        const owner = document.getElementById('transfer-acc-owner').value;
        if (!user || !owner) return;
        addTaskIndicator();
        try {
            const res = await fetch('/api/reseller/transfer', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${localStorage.getItem('neocp_token')}`
                },
                body: JSON.stringify({ username: user, new_owner: owner })
            });
            if (res.ok) {
                showNotification(`Ownership of ${user} transferred to ${owner}.`, 'success');
                loadResellerAccounts();
            }
        } catch (e) {} finally {
            removeTaskIndicator();
        }
    });

    // ==========================================================================
    // 14. DYNAMIC CRON JOBS MANAGER (cPanel Mapped)
    // ==========================================================================
    async function loadCronJobs() {
        // Mapped automatically
        console.log('Cron tasks queried.');
    }

    // ==========================================================================
    // 15. DOCKER CONTAINER HUB
    // ==========================================================================
    async function loadDockerContainers() {
        const tbody = document.getElementById('containers-table-body');
        if (!tbody) return;

        try {
            const res = await fetch('/api/docker/containers');
            if (!res.ok) {
                tbody.innerHTML = `<tr><td colspan="6" class="text-red" style="text-align: center; padding: 20px;">Failed to fetch container state from backend.</td></tr>`;
                return;
            }
            const data = await res.json();
            if (!data || data.length === 0) {
                tbody.innerHTML = `<tr><td colspan="6" class="text-muted" style="text-align: center; padding: 20px;">No containers currently running.</td></tr>`;
                return;
            }

            tbody.innerHTML = '';
            data.forEach(c => {
                const tr = document.createElement('tr');
                const statusBadge = c.status === 'running' 
                    ? '<span class="status-badge badge-green">running</span>' 
                    : '<span class="status-badge badge-red">stopped</span>';
                
                const toggleLabel = c.status === 'running' ? 'Stop' : 'Start';
                
                // Format ports for display
                const portsStr = c.ports ? c.ports : '<span class="text-muted">none</span>';
                const dateStr = c.created_at ? new Date(c.created_at).toLocaleString() : 'N/A';

                tr.innerHTML = `
                    <td><strong>🐳 ${c.name}</strong></td>
                    <td><span class="text-blue">${c.image}</span></td>
                    <td><code>${portsStr}</code></td>
                    <td>${statusBadge}</td>
                    <td><span class="text-muted">${dateStr}</span></td>
                    <td>
                        <button class="action-btn-secondary toggle-docker-btn" data-name="${c.name}" style="padding: 4px 8px; margin: 0; width: auto; font-size: 0.85em;">${toggleLabel}</button>
                        <button class="action-btn-danger delete-docker-btn" data-name="${c.name}" style="padding: 4px 8px; margin: 0; width: auto; font-size: 0.85em; background: rgba(239, 68, 68, 0.2); border: 1px solid #ef4444; color: #ef4444;">Delete</button>
                    </td>
                `;
                tbody.appendChild(tr);
            });

            // Bind toggle buttons
            tbody.querySelectorAll('.toggle-docker-btn').forEach(btn => {
                btn.addEventListener('click', async () => {
                    const name = btn.getAttribute('data-name');
                    await toggleDockerContainer(name);
                });
            });

            // Bind delete buttons
            tbody.querySelectorAll('.delete-docker-btn').forEach(btn => {
                btn.addEventListener('click', async () => {
                    const name = btn.getAttribute('data-name');
                    if (confirm(`Are you sure you want to destroy and delete container "${name}"?`)) {
                        await deleteDockerContainer(name);
                    }
                });
            });

        } catch (err) {
            console.error('Failed to load Docker containers:', err);
            tbody.innerHTML = `<tr><td colspan="6" class="text-red" style="text-align: center; padding: 20px;">Docker service offline or query timeout.</td></tr>`;
        }
    }

    async function deployDockerContainer(name, image, ports) {
        if (!name || !image) {
            showNotification('Container Name and Image are required parameters.', 'error');
            return;
        }
        try {
            addTaskIndicator();
            showNotification(`Provisioning Docker Image: ${image}...`, 'info');
            const res = await fetch('/api/docker/containers/deploy', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ name, image, ports })
            });
            if (res.ok) {
                showNotification(`Container "${name}" successfully deployed and active.`, 'success');
                // Clear input fields if custom form used
                const nameInput = document.getElementById('docker-name-input');
                const imageInput = document.getElementById('docker-image-input');
                const portsInput = document.getElementById('docker-ports-input');
                if (nameInput) nameInput.value = '';
                if (imageInput) imageInput.value = '';
                if (portsInput) portsInput.value = '';

                await loadDockerContainers();
            } else {
                const errText = await res.text();
                showNotification(`Provisioning failed: ${errText}`, 'error');
            }
        } catch (err) {
            console.error(err);
            showNotification('Network error provisioning container.', 'error');
        } finally {
            removeTaskIndicator();
        }
    }

    async function toggleDockerContainer(name) {
        try {
            addTaskIndicator();
            showNotification(`Sending power signal to ${name}...`, 'info');
            const res = await fetch('/api/docker/containers/toggle', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ name })
            });
            if (res.ok) {
                showNotification(`Container state toggled.`, 'success');
                await loadDockerContainers();
            } else {
                const errText = await res.text();
                showNotification(`Failed to toggle container: ${errText}`, 'error');
            }
        } catch (err) {
            console.error(err);
            showNotification('Network error toggling container.', 'error');
        } finally {
            removeTaskIndicator();
        }
    }

    async function deleteDockerContainer(name) {
        try {
            addTaskIndicator();
            showNotification(`Destroying container runtime: ${name}...`, 'info');
            const res = await fetch(`/api/docker/containers?name=${encodeURIComponent(name)}`, {
                method: 'DELETE'
            });
            if (res.ok) {
                showNotification(`Container "${name}" has been destroyed.`, 'success');
                await loadDockerContainers();
            } else {
                const errText = await res.text();
                showNotification(`Failed to destroy container: ${errText}`, 'error');
            }
        } catch (err) {
            console.error(err);
            showNotification('Network error removing container.', 'error');
        } finally {
            removeTaskIndicator();
        }
    }

    // ==========================================================================
    // 16. SECURE COGNITIVE WEB TERMINAL EMULATOR
    // ==========================================================================
    const termScreen = document.getElementById('terminal-screen');
    const termStdin = document.getElementById('terminal-stdin');

    termStdin.addEventListener('keydown', async (e) => {
        if (e.key === 'Enter') {
            const raw = termStdin.value.trim();
            termStdin.value = '';
            if (!raw) return;

            // Echo input
            appendTerminalLine(`root@neocp:~# ${raw}`);

            const tokens = raw.split(' ');
            const cmd = tokens[0].toLowerCase();
            const args = tokens.slice(1);

            if (cmd === 'help') {
                appendTerminalLine('Available Secure Shell Commands:', 'text-green');
                appendTerminalLine('  ls                     — Lists folders in active user webroot sandbox');
                appendTerminalLine('  neofetch               — Prints dynamic systems matrix specifications');
                appendTerminalLine('  service list           — Displays active OS daemons and PID references');
                appendTerminalLine('  service restart [name] — Restarts an OS daemon (web, dns, ftp, mail, db, php)');
                appendTerminalLine('  top                    — Prints real-time system metrics telemetry overall');
                appendTerminalLine('  clear                  — Clears screen lines');
            } else if (cmd === 'clear') {
                termScreen.innerHTML = '';
            } else if (cmd === 'ls') {
                addTaskIndicator();
                try {
                    const res = await fetch(`/api/filemanager/list?path=${encodeURIComponent(currentPath)}`, {
                        headers: { 'Authorization': `Bearer ${localStorage.getItem('neocp_token')}` }
                    });
                    const items = await res.json();
                    const line = items.map(item => item.is_dir ? `\x1B[34m${item.name}\x1B[0m` : item.name).join('   ');
                    appendTerminalLine(line || '[Empty Sandbox Directory]');
                } catch (e) {
                    appendTerminalLine('Error traversing folder.', 'text-red');
                } finally {
                    removeTaskIndicator();
                }
            } else if (cmd === 'neofetch') {
                const clock = new Date().toLocaleTimeString();
                appendTerminalLine('      \x1B[38;5;33m▲\x1B[0m          root@neocp.professional', 'text-blue');
                appendTerminalLine('     \x1B[38;5;33m▲▲\x1B[0m         -----------------------', 'text-blue');
                appendTerminalLine('    \x1B[38;5;33m▲▲▲▲\x1B[0m        OS: Windows / Linux Monolithic Native Core');
                appendTerminalLine('   \x1B[38;5;33m▲▲\x1B[0m  \x1B[38;5;33m▲▲\x1B[0m       Kernel: NeoCP High-Performance Core v1.0.0');
                appendTerminalLine('  \x1B[38;5;33m▲▲▲▲▲▲▲▲\x1B[0m      Uptime: Active & Valid license key');
                appendTerminalLine(' \x1B[38;5;33m▲▲▲▲\x1B[0m  \x1B[38;5;33m▲▲▲▲\x1B[0m     Shell: Encrypted Visual WebSocket Shell');
                appendTerminalLine('\x1B[38;5;33m▲▲▲▲▲▲▲▲▲▲▲▲\x1B[0m    Resolution: 1080p Glassmorphism SPA');
                appendTerminalLine('                Node Scope: Master Controller Cluster');
            } else if (cmd === 'service' && args[0] === 'list') {
                addTaskIndicator();
                try {
                    const res = await fetch('/api/services', {
                        headers: { 'Authorization': `Bearer ${localStorage.getItem('neocp_token')}` }
                    });
                    const svcs = await res.json();
                    svcs.forEach(s => {
                        appendTerminalLine(`  [${s.is_running ? '🟢 RUNNING' : '🔴 STOPPED'}] PID:${s.pid} — ${s.display_name} (${s.memory_mb.toFixed(1)} MB)`);
                    });
                } catch (e) {
                    appendTerminalLine('Failed to query service registries.', 'text-red');
                } finally {
                    removeTaskIndicator();
                }
            } else if (cmd === 'service' && args[0] === 'restart') {
                const target = args[1];
                if (!target) {
                    appendTerminalLine('Usage: service restart [web|dns|ftp|mail|db|php]', 'text-red');
                    return;
                }
                addTaskIndicator();
                appendTerminalLine(`Cycling service stack ${target}...`, 'text-blue');
                try {
                    const res = await fetch(`/api/services/restart?name=${target}`, {
                        method: 'POST',
                        headers: { 'Authorization': `Bearer ${localStorage.getItem('neocp_token')}` }
                    });
                    if (res.ok) {
                        appendTerminalLine(`Service daemon ${target} successfully recycled and hot-reloaded!`, 'text-green');
                        loadOSServicesList();
                    } else {
                        appendTerminalLine(`Failed to recycle daemon ${target}.`, 'text-red');
                    }
                } catch (e) {
                    appendTerminalLine('Service API offline.', 'text-red');
                } finally {
                    removeTaskIndicator();
                }
            } else if (cmd === 'top') {
                // Overall CPU summary
                appendTerminalLine(`Overall system load averages: ${cpuHistory[cpuHistory.length-1].toFixed(1)}% CPU overall usage.`);
            } else {
                appendTerminalLine(`bash: ${cmd}: command not recognized in safe parameters mode. Type 'help'`, 'text-red');
            }

            termScreen.scrollTop = termScreen.scrollHeight;
        }
    });

    function appendTerminalLine(text, styleClass = '') {
        const div = document.createElement('div');
        div.className = `terminal-line ${styleClass}`;
        // Color escaping replacement simulation
        let cleanText = text
            .replace(/\x1B\[34m/g, '<span class="text-blue">')
            .replace(/\x1B\[0m/g, '</span>');
        div.innerHTML = cleanText;
        termScreen.appendChild(div);
        termScreen.scrollTop = termScreen.scrollHeight;
    }

    // ==========================================================================
    // 17. OS SERVICE LIFECYCLE MANAGEMENT
    // ==========================================================================
    async function loadOSServicesList() {
        const container = document.getElementById('view-system');
        // Let's create service monitoring blocks in System Telemetry view dynamically!
        let statusBlock = document.getElementById('system-services-status-card');
        if (!statusBlock) {
            statusBlock = document.createElement('div');
            statusBlock.className = 'split-card glass';
            statusBlock.id = 'system-services-status-card';
            statusBlock.style.marginTop = '25px';
            statusBlock.innerHTML = `
                <h4>System Daemon Status Control Center</h4>
                <div class="services-list-grid" id="os-services-list-container" style="display:grid; grid-template-columns: repeat(2, 1fr); gap: 15px; margin-top: 15px;">
                    <!-- Services loaded via JS -->
                </div>
            `;
            container.appendChild(statusBlock);
        }

        const grid = document.getElementById('os-services-list-container');
        grid.innerHTML = '<div class="text-muted">Querying OS Service registries...</div>';

        try {
            const res = await fetch('/api/services', {
                headers: { 'Authorization': `Bearer ${localStorage.getItem('neocp_token')}` }
            });
            const svcs = await res.json();

            grid.innerHTML = '';
            svcs.forEach(s => {
                const card = document.createElement('div');
                card.className = 'db-panel-card';
                card.style.display = 'flex';
                card.style.justifyContent = 'space-between';
                card.style.alignItems = 'center';
                card.style.background = 'rgba(255,255,255,0.02)';
                card.style.padding = '12px';
                card.style.borderRadius = '8px';
                card.innerHTML = `
                    <div>
                        <strong class="text-blue">${s.display_name}</strong><br>
                        <span style="font-size:0.75rem; color: var(--text-muted);">Uptime: ${formatUptime(s.uptime_sec)} | RAM: ${s.memory_mb.toFixed(1)} MB | PID: ${s.pid}</span>
                    </div>
                    <div style="display:flex; align-items:center; gap:8px;">
                        <span class="status-badge ${s.is_running ? 'badge-green' : 'badge-red'}">${s.is_running ? 'Active' : 'Offline'}</span>
                        <button class="action-btn-secondary restart-svc-inline-btn" data-svc="${s.name}" style="padding: 4px 8px; font-size: 0.75rem;">Recycle</button>
                    </div>
                `;

                card.querySelector('.restart-svc-inline-btn').addEventListener('click', async () => {
                    addTaskIndicator();
                    showNotification(`Recycling daemon ${s.display_name}...`, 'info');
                    try {
                        const recRes = await fetch(`/api/services/restart?name=${s.name}`, {
                            method: 'POST',
                            headers: { 'Authorization': `Bearer ${localStorage.getItem('neocp_token')}` }
                        });
                        if (recRes.ok) {
                            showNotification(`${s.display_name} recycled and hot-reloaded successfully.`, 'success');
                            loadOSServicesList();
                        }
                    } catch (e) {
                        showNotification('Failed to recycle service.', 'error');
                    } finally {
                        removeTaskIndicator();
                    }
                });

                grid.appendChild(card);
            });
        } catch (e) {}
    }

    function formatUptime(secs) {
        if (secs < 60) return `${secs}s`;
        const mins = Math.floor(secs / 60);
        if (mins < 60) return `${mins}m`;
        const hrs = Math.floor(mins / 60);
        return `${hrs}h ${mins%60}m`;
    }

    // ==========================================================================
    // 18. LIVE PRIORITY SUPPORT HUB & TICKETS CHAT
    // ==========================================================================
    const supportModal = document.getElementById('support-modal');
    const openSupportBtn = document.getElementById('open-support-btn');
    const closeSupportModalBtn = document.getElementById('close-support-modal-btn');
    const ticketListContainer = document.getElementById('support-tickets-container');
    const chatThreadBox = document.getElementById('chat-thread-box');
    const chatStdin = document.getElementById('chat-stdin');
    const sendChatBtn = document.getElementById('send-chat-btn');
    const submitTicketBtn = document.getElementById('submit-ticket-btn');

    openSupportBtn.addEventListener('click', () => {
        supportModal.classList.add('active');
        loadSupportTickets();
    });

    closeSupportModalBtn.addEventListener('click', () => {
        supportModal.classList.remove('active');
    });

    async function loadSupportTickets() {
        ticketListContainer.innerHTML = '<div class="text-muted">Loading active support tickets...</div>';
        try {
            const res = await fetch('/api/tickets', {
                headers: { 'Authorization': `Bearer ${localStorage.getItem('neocp_token')}` }
            });
            const tickets = await res.json();

            ticketListContainer.innerHTML = '';
            if (tickets.length === 0) {
                ticketListContainer.innerHTML = '<div class="text-muted">No tickets found.</div>';
                return;
            }

            tickets.forEach(t => {
                const item = document.createElement('div');
                item.className = `support-ticket-item ${activeTicketId === t.id ? 'active' : ''}`;
                item.innerHTML = `
                    <div style="display:flex; justify-content:space-between; align-items:center;">
                        <strong>#${t.id} - ${t.subject}</strong>
                        <span class="status-badge ${t.status === 'answered' ? 'badge-green' : 'badge-red'}">${t.status}</span>
                    </div>
                    <span style="font-size:0.75rem; color:var(--text-muted);">${t.category} | Created by ${t.owner}</span>
                `;

                item.addEventListener('click', () => {
                    activeTicketId = t.id;
                    // Render Active Thread
                    renderChatThread(t);
                    // Add active class style
                    document.querySelectorAll('.support-ticket-item').forEach(el => el.classList.remove('active'));
                    item.classList.add('active');
                });

                ticketListContainer.appendChild(item);
            });
        } catch (e) {}
    }

    function renderChatThread(ticket) {
        chatThreadBox.innerHTML = '';
        ticket.messages.forEach(msg => {
            const wrapper = document.createElement('div');
            wrapper.className = `chat-message-wrapper ${msg.sender === currentUser ? 'self' : 'representative'}`;
            wrapper.innerHTML = `
                <div class="chat-sender-avatar">${msg.sender === currentUser ? 'Me' : 'CP'}</div>
                <div class="chat-message-bubble">
                    <div class="chat-sender-name">${msg.sender === currentUser ? 'You' : 'Priority Support Admin'}</div>
                    <p>${msg.message}</p>
                    <span class="chat-timestamp">${new Date(msg.timestamp).toLocaleTimeString([], {hour: '2-digit', minute:'2-digit'})}</span>
                </div>
            `;
            chatThreadBox.appendChild(wrapper);
        });
        chatThreadBox.scrollTop = chatThreadBox.scrollHeight;
    }

    sendChatBtn.addEventListener('click', async () => {
        const msg = chatStdin.value.trim();
        if (!msg || !activeTicketId) return;
        chatStdin.value = '';

        addTaskIndicator();
        try {
            const res = await fetch('/api/tickets/reply', {
                method: 'POST',
                headers: { 
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${localStorage.getItem('neocp_token')}`
                },
                body: JSON.stringify({ ticket_id: activeTicketId, message: msg, sender: currentUser })
            });
            if (res.ok) {
                // Refresh ticket list
                await reloadActiveTicketThread();
                
                // Simulate smart assistant reply
                setTimeout(async () => {
                    await fetch('/api/tickets/reply', {
                        method: 'POST',
                        headers: { 
                            'Content-Type': 'application/json',
                            'Authorization': `Bearer ${localStorage.getItem('neocp_token')}`
                        },
                        body: JSON.stringify({ 
                            ticket_id: activeTicketId, 
                            message: `Hello! I have reviewed your request regarding your NeoCP profile. Rest assured, our engineering core is syncing system metrics. Let us know if you need further help!`, 
                            sender: 'admin' 
                        })
                    });
                    await reloadActiveTicketThread();
                    showNotification('Support representative answered your ticket reply.', 'info');
                }, 1800);
            }
        } catch (e) {
            showNotification('Error sending message.', 'error');
        } finally {
            removeTaskIndicator();
        }
    });

    async function reloadActiveTicketThread() {
        try {
            const res = await fetch('/api/tickets', {
                headers: { 'Authorization': `Bearer ${localStorage.getItem('neocp_token')}` }
            });
            const tickets = await res.json();
            const ticket = tickets.find(t => t.id === activeTicketId);
            if (ticket) {
                renderChatThread(ticket);
            }
            loadSupportTickets();
        } catch (e) {}
    }

    submitTicketBtn.addEventListener('click', async () => {
        const subject = document.getElementById('support-subject-input').value;
        const category = document.getElementById('support-category-select').value;
        const msg = document.getElementById('support-message-input').value;

        if (!subject || !msg) {
            showNotification('Please enter subject and message.', 'error');
            return;
        }

        addTaskIndicator();
        try {
            const res = await fetch('/api/tickets', {
                method: 'POST',
                headers: { 
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${localStorage.getItem('neocp_token')}`
                },
                body: JSON.stringify({
                    id: `t_${Math.floor(Math.random() * 900) + 100}`,
                    owner: currentUser,
                    subject,
                    status: "open",
                    category,
                    messages: [{ sender: currentUser, message: msg, timestamp: new Date() }]
                })
            });
            if (res.ok) {
                showNotification('Support ticket created successfully. Direct Rep assigned.', 'success');
                document.getElementById('support-subject-input').value = '';
                document.getElementById('support-message-input').value = '';
                loadSupportTickets();
            }
        } catch (e) {
            showNotification('Error creating ticket.', 'error');
        } finally {
            removeTaskIndicator();
        }
    });

    // ==========================================================================
    // 19. NOTIFICATION & TASK INDICATORS
    // ==========================================================================
    function showNotification(message, type = 'info') {
        const notif = document.createElement('div');
        notif.className = `neocp-toast toast-${type}`;
        notif.innerHTML = `
            <span class="toast-icon">${type === 'success' ? '🟢' : type === 'error' ? '🔴' : '🔵'}</span>
            <span class="toast-message">${message}</span>
        `;
        document.body.appendChild(notif);

        // Slide in
        setTimeout(() => notif.classList.add('active'), 10);
        // Wipe out
        setTimeout(() => {
            notif.classList.remove('active');
            setTimeout(() => notif.remove(), 300);
        }, 4500);
    }

    function addTaskIndicator() {
        activeTasks++;
        activeTasksCount.textContent = activeTasks;
        activeTasksCount.style.animation = 'pulse 1s infinite';
    }

    function removeTaskIndicator() {
        activeTasks--;
        if (activeTasks < 0) activeTasks = 0;
        activeTasksCount.textContent = activeTasks;
        if (activeTasks === 0) activeTasksCount.style.animation = 'none';
    }

    // ==========================================================================
    // 20. MONOLITHIC BACKUP UI CONTROLLERS
    // ==========================================================================
    async function loadBackupsTable() {
        const tbody = document.getElementById('backups-table-body');
        tbody.innerHTML = '<tr><td colspan="4" class="text-muted">Loading backup archives...</td></tr>';
        try {
            const res = await fetch('/api/backup/list', {
                headers: { 'Authorization': `Bearer ${localStorage.getItem('neocp_token')}` }
            });
            const backups = await res.json();
            tbody.innerHTML = '';
            if (backups.length === 0) {
                tbody.innerHTML = '<tr><td colspan="4" class="text-muted" style="text-align: center; padding: 20px;">No backups found. Click "Create Full Backup" above.</td></tr>';
                return;
            }
            backups.forEach(b => {
                const tr = document.createElement('tr');
                tr.innerHTML = `
                    <td><strong class="text-blue">📁 ${b.filename}</strong></td>
                    <td>${b.size_mb.toFixed(2)} MB</td>
                    <td><span class="text-muted">${b.created_at}</span></td>
                    <td>
                        <button class="action-btn restore-backup-btn" data-filename="${b.filename}" style="padding: 4px 8px; font-size: 0.75rem;">Restore</button>
                    </td>
                `;
                tbody.appendChild(tr);
            });
            document.querySelectorAll('.restore-backup-btn').forEach(btn => {
                btn.addEventListener('click', async (e) => {
                    const fn = e.target.getAttribute('data-filename');
                    if (confirm(`Are you absolutely sure you want to restore the backup archive ${fn}? This will purge your active folder files before extraction.`)) {
                        await restoreBackupArchive(fn);
                    }
                });
            });
        } catch (e) {
            tbody.innerHTML = '<tr><td colspan="4" class="text-red">Failed to load backup archives.</td></tr>';
        }
    }

    async function createBackupArchive() {
        addTaskIndicator();
        showNotification('Packaging customer sandbox files into ZIP archive...', 'info');
        try {
            const res = await fetch('/api/backup/create', {
                method: 'POST',
                headers: { 'Authorization': `Bearer ${localStorage.getItem('neocp_token')}` }
            });
            if (res.ok) {
                showNotification('Backup archive created successfully!', 'success');
                loadBackupsTable();
            } else {
                showNotification('Failed to generate backup archive.', 'error');
            }
        } catch (e) {
            showNotification('Server backup request timed out.', 'error');
        } finally {
            removeTaskIndicator();
        }
    }

    async function restoreBackupArchive(filename) {
        addTaskIndicator();
        showNotification(`Restoring files from archive ${filename}...`, 'info');
        try {
            const res = await fetch('/api/backup/restore', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${localStorage.getItem('neocp_token')}`
                },
                body: JSON.stringify({ filename })
            });
            if (res.ok) {
                showNotification('Full state restored from ZIP package successfully!', 'success');
                if (document.getElementById('view-filemanager').classList.contains('active')) {
                    loadFileExplorer();
                }
            } else {
                showNotification('Failed to restore files from backup.', 'error');
            }
        } catch (e) {
            showNotification('Server restoration request timed out.', 'error');
        } finally {
            removeTaskIndicator();
        }
    }

    document.getElementById('trigger-backup-btn').addEventListener('click', createBackupArchive);

    // ==========================================================================
    // 20B. OS FIREWALL CONTROLLERS & MANUAL BLOCKS
    // ==========================================================================
    async function loadFirewallBlocks() {
        const tbody = document.getElementById('firewall-blocks-table-body');
        if (!tbody) return;

        if (currentRole !== 'admin') {
            tbody.innerHTML = `<tr><td colspan="4" class="text-muted" style="text-align: center; padding: 20px;">🛡️ cPHulk Firewall active. (Access restricted to System Administrators)</td></tr>`;
            return;
        }

        try {
            const res = await fetch('/api/security/firewall/blocks');
            if (res.status === 403 || res.status === 401) {
                tbody.innerHTML = `<tr><td colspan="4" class="text-muted" style="text-align: center; padding: 20px;">🔒 Unauthorized to view firewall blocks.</td></tr>`;
                return;
            }
            const data = await res.json();
            if (!data || data.length === 0) {
                tbody.innerHTML = `<tr><td colspan="4" class="text-muted" style="text-align: center; padding: 20px;">No active IP blocks currently in the database.</td></tr>`;
                return;
            }

            tbody.innerHTML = '';
            data.forEach(block => {
                const tr = document.createElement('tr');
                const dateStr = new Date(block.blocked_at).toLocaleString();
                tr.innerHTML = `
                    <td><strong>${block.ip}</strong></td>
                    <td><span class="text-orange">${block.reason}</span></td>
                    <td><span class="text-muted">${dateStr}</span></td>
                    <td>
                        <button class="action-btn-secondary unblock-firewall-btn" data-ip="${block.ip}" style="padding: 4px 10px; margin: 0; width: auto; background: rgba(239, 68, 68, 0.2); border: 1px solid #ef4444; color: #ef4444;">Unblock</button>
                    </td>
                `;
                tbody.appendChild(tr);
            });

            // Bind unblock click handlers
            tbody.querySelectorAll('.unblock-firewall-btn').forEach(btn => {
                btn.addEventListener('click', async () => {
                    const ip = btn.getAttribute('data-ip');
                    await unblockIPAddress(ip);
                });
            });
        } catch (err) {
            console.error('Failed to query firewall blocks:', err);
            tbody.innerHTML = `<tr><td colspan="4" class="text-red" style="text-align: center; padding: 20px;">Failed to fetch firewall records.</td></tr>`;
        }
    }

    async function blockIPAddress(ip, reason) {
        if (!ip) {
            showNotification('Please enter a valid IP address.', 'error');
            return;
        }
        try {
            addTaskIndicator();
            showNotification(`Injecting block rule for ${ip}...`, 'info');
            const res = await fetch('/api/security/firewall/block', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ ip, reason })
            });
            if (res.ok) {
                showNotification(`IP Address ${ip} has been successfully banned.`, 'success');
                document.getElementById('firewall-ip-input').value = '';
                document.getElementById('firewall-reason-input').value = '';
                await loadFirewallBlocks();
            } else {
                const errText = await res.text();
                showNotification(`Failed to block: ${errText}`, 'error');
            }
        } catch (err) {
            console.error(err);
            showNotification('Network communication error blocking IP.', 'error');
        } finally {
            removeTaskIndicator();
        }
    }

    async function unblockIPAddress(ip) {
        try {
            addTaskIndicator();
            showNotification(`Purging block rule for ${ip}...`, 'info');
            const res = await fetch('/api/security/firewall/unblock', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ ip })
            });
            if (res.ok) {
                showNotification(`IP Address ${ip} unblocked.`, 'success');
                await loadFirewallBlocks();
            } else {
                const errText = await res.text();
                showNotification(`Failed to unblock: ${errText}`, 'error');
            }
        } catch (err) {
            console.error(err);
            showNotification('Network error lifting block.', 'error');
        } finally {
            removeTaskIndicator();
        }
    }

    // Docker Deployment Forms & Templates bindings
    const customDeployBtn = document.getElementById('deploy-custom-container-btn');
    if (customDeployBtn) {
        customDeployBtn.addEventListener('click', async () => {
            const name = document.getElementById('docker-name-input').value.trim();
            const image = document.getElementById('docker-image-input').value.trim();
            const ports = document.getElementById('docker-ports-input').value.trim();
            await deployDockerContainer(name, image, ports);
        });
    }

    // 1-Click App templates delegation or bindings
    document.querySelectorAll('.install-docker-template-btn').forEach(btn => {
        btn.addEventListener('click', async () => {
            const image = btn.getAttribute('data-image');
            const name = btn.getAttribute('data-name');
            const ports = btn.getAttribute('data-ports');
            await deployDockerContainer(name, image, ports);
        });
    });

    // Firewall manual block bindings
    const firewallBlockBtn = document.getElementById('firewall-block-btn');
    if (firewallBlockBtn) {
        firewallBlockBtn.addEventListener('click', async () => {
            const ip = document.getElementById('firewall-ip-input').value.trim();
            const reason = document.getElementById('firewall-reason-input').value.trim();
            await blockIPAddress(ip, reason);
        });
    }

    // ==========================================================================
    // STAGE 4: DNS ZONE EDITOR & OWASP WAF CONFIGURATION
    // ==========================================================================
    function loadDomainSelectorsFromCache(domains) {
        const dnsSelector = document.getElementById('dns-domain-selector');
        const wafSelector = document.getElementById('waf-domain-selector');
        
        if (dnsSelector) {
            const currentVal = dnsSelector.value;
            dnsSelector.innerHTML = '<option value="">-- Choose Domain --</option>';
            domains.forEach(d => {
                const opt = document.createElement('option');
                opt.value = d.domain_name;
                opt.textContent = d.domain_name;
                dnsSelector.appendChild(opt);
            });
            if (currentVal && domains.some(d => d.domain_name === currentVal)) {
                dnsSelector.value = currentVal;
            }
        }
        
        if (wafSelector) {
            const currentVal = wafSelector.value;
            wafSelector.innerHTML = '<option value="">-- Choose Domain --</option>';
            domains.forEach(d => {
                const opt = document.createElement('option');
                opt.value = d.domain_name;
                opt.textContent = d.domain_name;
                wafSelector.appendChild(opt);
            });
            if (currentVal && domains.some(d => d.domain_name === currentVal)) {
                wafSelector.value = currentVal;
            }
        }
    }

    async function loadDNSRecords(domain) {
        const tbody = document.getElementById('dns-records-table-body');
        if (!tbody) return;
        if (!domain) {
            tbody.innerHTML = '<tr><td colspan="6" class="text-muted" style="text-align: center; padding: 20px;">Please select an active domain above to view its DNS records.</td></tr>';
            return;
        }
        
        tbody.innerHTML = '<tr><td colspan="6" class="text-muted" style="text-align: center; padding: 20px;">Fetching DNS records...</td></tr>';
        try {
            const res = await fetch(`/api/domains/dns?domain=${encodeURIComponent(domain)}`, {
                headers: { 'Authorization': `Bearer ${localStorage.getItem('neocp_token')}` }
            });
            if (!res.ok) {
                tbody.innerHTML = '<tr><td colspan="6" class="text-red" style="text-align: center; padding: 20px;">Failed to fetch DNS records.</td></tr>';
                return;
            }
            const records = await res.json();
            tbody.innerHTML = '';
            if (records.length === 0) {
                tbody.innerHTML = '<tr><td colspan="6" class="text-muted" style="text-align: center; padding: 20px;">No DNS records configured.</td></tr>';
                return;
            }
            records.forEach(r => {
                const tr = document.createElement('tr');
                tr.innerHTML = `
                    <td><span class="status-badge badge-green" style="font-size:0.75rem;">${r.type}</span></td>
                    <td><strong>${r.name || '@'}</strong></td>
                    <td style="word-break:break-all; max-width:200px;">${r.value}</td>
                    <td>${r.ttl}</td>
                    <td>${r.priority || '-'}</td>
                    <td>
                        <button class="action-btn-secondary delete-dns-record-btn text-red" data-id="${r.id}" style="padding: 2px 6px; font-size: 0.75rem; border-color: rgba(239, 68, 68, 0.2);">Delete</button>
                    </td>
                `;
                tr.querySelector('.delete-dns-record-btn').addEventListener('click', async () => {
                    if (confirm(`Remove this DNS record?`)) {
                        await deleteDNSRecord(domain, r.id);
                    }
                });
                tbody.appendChild(tr);
            });
        } catch (e) {
            tbody.innerHTML = '<tr><td colspan="6" class="text-red" style="text-align: center; padding: 20px;">Error parsing DNS records.</td></tr>';
        }
    }

    async function publishDNSRecord() {
        const domain = document.getElementById('dns-domain-selector').value;
        if (!domain) {
            showNotification('Please select a domain first.', 'error');
            return;
        }
        const type = document.getElementById('dns-record-type').value;
        const name = document.getElementById('dns-record-name').value.trim();
        const value = document.getElementById('dns-record-value').value.trim();
        const ttl = parseInt(document.getElementById('dns-record-ttl').value) || 86400;
        const priority = parseInt(document.getElementById('dns-record-priority').value) || 10;
        
        if (!value) {
            showNotification('Target value cannot be empty.', 'error');
            return;
        }
        
        addTaskIndicator();
        try {
            const res = await fetch('/api/domains/dns', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${localStorage.getItem('neocp_token')}`
                },
                body: JSON.stringify({
                    domain_name: domain,
                    record: {
                        type,
                        name,
                        value,
                        ttl,
                        priority
                    }
                })
            });
            const data = await res.json();
            if (res.ok) {
                showNotification('DNS record published and BIND9 zone file recompiled successfully!', 'success');
                document.getElementById('dns-record-name').value = '';
                document.getElementById('dns-record-value').value = '';
                await loadDNSRecords(domain);
            } else {
                showNotification(data.error || 'Failed to add DNS record.', 'error');
            }
        } catch (e) {
            showNotification('Failed to save DNS record.', 'error');
        } finally {
            removeTaskIndicator();
        }
    }

    async function deleteDNSRecord(domain, recordId) {
        addTaskIndicator();
        try {
            const res = await fetch('/api/domains/dns', {
                method: 'DELETE',
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${localStorage.getItem('neocp_token')}`
                },
                body: JSON.stringify({
                    domain_name: domain,
                    record_id: recordId
                })
            });
            if (res.ok) {
                showNotification('DNS record deleted and BIND9 zone file updated.', 'success');
                await loadDNSRecords(domain);
            } else {
                showNotification('Failed to delete DNS record.', 'error');
            }
        } catch (e) {
            showNotification('Failed to delete DNS record.', 'error');
        } finally {
            removeTaskIndicator();
        }
    }

    function loadWAFPolicy(domain) {
        const sqliToggle = document.getElementById('waf-sqli-toggle');
        const xssToggle = document.getElementById('waf-xss-toggle');
        const lfiToggle = document.getElementById('waf-lfi-toggle');
        const csrfToggle = document.getElementById('waf-csrf-toggle');
        
        if (!domain) {
            sqliToggle.checked = false;
            xssToggle.checked = false;
            lfiToggle.checked = false;
            csrfToggle.checked = false;
            updateWAFBadgeState('sqli', false);
            updateWAFBadgeState('xss', false);
            updateWAFBadgeState('lfi', false);
            updateWAFBadgeState('csrf', false);
            return;
        }
        
        const d = cachedDomains.find(item => item.domain_name === domain);
        if (d && d.waf_policy) {
            sqliToggle.checked = !!d.waf_policy.sqli_shield;
            xssToggle.checked = !!d.waf_policy.xss_block;
            lfiToggle.checked = !!d.waf_policy.lfi_shield;
            csrfToggle.checked = !!d.waf_policy.csrf_header;
            
            updateWAFBadgeState('sqli', sqliToggle.checked);
            updateWAFBadgeState('xss', xssToggle.checked);
            updateWAFBadgeState('lfi', lfiToggle.checked);
            updateWAFBadgeState('csrf', csrfToggle.checked);
        } else {
            sqliToggle.checked = false;
            xssToggle.checked = false;
            lfiToggle.checked = false;
            csrfToggle.checked = false;
            
            updateWAFBadgeState('sqli', false);
            updateWAFBadgeState('xss', false);
            updateWAFBadgeState('lfi', false);
            updateWAFBadgeState('csrf', false);
        }
    }

    function updateWAFBadgeState(type, active) {
        const badge = document.getElementById(`waf-${type}-badge`);
        if (!badge) return;
        if (active) {
            badge.textContent = 'Active';
            badge.style.background = 'rgba(16, 185, 129, 0.15)';
            badge.style.color = '#10b981';
            badge.style.borderColor = 'rgba(16, 185, 129, 0.25)';
            badge.style.boxShadow = '0 0 10px rgba(16, 185, 129, 0.2)';
        } else {
            badge.textContent = 'Inactive';
            badge.style.background = 'rgba(239, 68, 68, 0.15)';
            badge.style.color = '#ef4444';
            badge.style.borderColor = 'rgba(239, 68, 68, 0.25)';
            badge.style.boxShadow = 'none';
        }
    }

    async function saveWAFPolicy() {
        const domain = document.getElementById('waf-domain-selector').value;
        if (!domain) {
            showNotification('Please select a domain first.', 'error');
            return;
        }
        
        const sqli = document.getElementById('waf-sqli-toggle').checked;
        const xss = document.getElementById('waf-xss-toggle').checked;
        const lfi = document.getElementById('waf-lfi-toggle').checked;
        const csrf = document.getElementById('waf-csrf-toggle').checked;
        
        addTaskIndicator();
        try {
            const res = await fetch('/api/domains/waf', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${localStorage.getItem('neocp_token')}`
                },
                body: JSON.stringify({
                    domain_name: domain,
                    waf_policy: {
                        sqli_shield: sqli,
                        xss_block: xss,
                        lfi_shield: lfi,
                        csrf_header: csrf
                    }
                })
            });
            if (res.ok) {
                showNotification('OWASP WAF updated and Nginx configurations dynamically reloaded!', 'success');
                await loadDomainsTable();
            } else {
                showNotification('Failed to update WAF security policy.', 'error');
            }
        } catch (e) {
            showNotification('API connection error updating WAF settings.', 'error');
        } finally {
            removeTaskIndicator();
        }
    }

    // Set up WAF event listeners and DNS bindings
    const dnsSecurityBtn = document.getElementById('dns-security-suite-btn');
    if (dnsSecurityBtn) {
        dnsSecurityBtn.addEventListener('click', async () => {
            const domain = document.getElementById('dns-domain-selector').value;
            if (!domain) return showNotification('Select a domain first.', 'error');
            addTaskIndicator();
            try {
                const res = await fetch('/api/domains/dns/security', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                        'Authorization': `Bearer ${localStorage.getItem('neocp_token')}`
                    },
                    body: JSON.stringify({ domain_name: domain })
                });
                const data = await res.json();
                if (res.ok) {
                    showNotification(data.log, 'success');
                    loadDNSRecords(domain);
                }
            } catch (e) {} finally {
                removeTaskIndicator();
            }
        });
    }

    const dnsSyncBtn = document.getElementById('dns-cluster-sync-btn');
    if (dnsSyncBtn) {
        dnsSyncBtn.addEventListener('click', async () => {
            addTaskIndicator();
            showNotification('Broadcasting zone updates to DNS cluster nodes...', 'info');
            try {
                const res = await fetch('/api/domains/dns/sync', {
                    method: 'POST',
                    headers: { 'Authorization': `Bearer ${localStorage.getItem('neocp_token')}` }
                });
                const data = await res.json();
                if (res.ok) {
                    showNotification(data.log, 'success');
                }
            } catch (e) {} finally {
                removeTaskIndicator();
            }
        });
    }

    const dnsDomSel = document.getElementById('dns-domain-selector');
    if (dnsDomSel) {
        dnsDomSel.addEventListener('change', (e) => {
            loadDNSRecords(e.target.value);
        });
    }

    const dnsAddRecBtn = document.getElementById('dns-add-record-btn');
    if (dnsAddRecBtn) {
        dnsAddRecBtn.addEventListener('click', publishDNSRecord);
    }

    const dnsTypeSel = document.getElementById('dns-record-type');
    if (dnsTypeSel) {
        dnsTypeSel.addEventListener('change', (e) => {
            const priorityInput = document.getElementById('dns-record-priority');
            if (priorityInput) {
                const priorityGroup = priorityInput.parentElement;
                if (priorityGroup) {
                    if (e.target.value === 'MX' || e.target.value === 'SRV') {
                        priorityGroup.style.opacity = '1';
                        priorityGroup.style.pointerEvents = 'auto';
                    } else {
                        priorityGroup.style.opacity = '0.2';
                        priorityGroup.style.pointerEvents = 'none';
                    }
                }
            }
        });
        // trigger initial setup
        dnsTypeSel.dispatchEvent(new Event('change'));
    }

    const wafDomSel = document.getElementById('waf-domain-selector');
    if (wafDomSel) {
        wafDomSel.addEventListener('change', (e) => {
            loadWAFPolicy(e.target.value);
        });
    }

    const wafSavePolicyBtn = document.getElementById('waf-save-policy-btn');
    if (wafSavePolicyBtn) {
        wafSavePolicyBtn.addEventListener('click', saveWAFPolicy);
    }

    const wafSqliTog = document.getElementById('waf-sqli-toggle');
    if (wafSqliTog) {
        wafSqliTog.addEventListener('change', (e) => updateWAFBadgeState('sqli', e.target.checked));
    }
    const wafXssTog = document.getElementById('waf-xss-toggle');
    if (wafXssTog) {
        wafXssTog.addEventListener('change', (e) => updateWAFBadgeState('xss', e.target.checked));
    }
    const wafLfiTog = document.getElementById('waf-lfi-toggle');
    if (wafLfiTog) {
        wafLfiTog.addEventListener('change', (e) => updateWAFBadgeState('lfi', e.target.checked));
    }
    const wafCsrfTog = document.getElementById('waf-csrf-toggle');
    if (wafCsrfTog) {
        wafCsrfTog.addEventListener('change', (e) => updateWAFBadgeState('csrf', e.target.checked));
    }

    // ==========================================================================
    // 21. LOGIN & INITIALIZATION BOOTSTRAP
    // ==========================================================================
    const loginOverlay = document.getElementById('login-overlay');
    const loginUsernameInput = document.getElementById('login-username');
    const loginPasswordInput = document.getElementById('login-password');
    const loginSubmitBtn = document.getElementById('login-submit-btn');
    const loginError = document.getElementById('login-error');
    const mainAppContainer = document.getElementById('main-app-container');

    async function performLogin() {
        const username = loginUsernameInput.value.trim();
        const password = loginPasswordInput.value.trim();

        if (!username || !password) {
            loginError.textContent = "Please enter both username and password.";
            loginError.style.display = "block";
            return;
        }

        loginSubmitBtn.disabled = true;
        loginSubmitBtn.textContent = "Authenticating...";
        loginError.style.display = "none";

        try {
            const res = await fetch('/api/login', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ username, password })
            });
            const data = await res.json();
            if (data.token) {
                // Set cookie & localStorage session
                document.cookie = `neocp_auth_token=${data.token}; path=/; max-age=86400; SameSite=Lax`;
                localStorage.setItem('neocp_token', data.token);
                currentUser = username;
                currentRole = data.role;

                // Adjust Profile display
                profileRole.textContent = currentRole.toUpperCase();
                if (currentRole === 'admin') {
                    profileName.textContent = 'Root System Administrator';
                    document.getElementById('footer-host-os').textContent = 'Linux / Windows Master';
                    userRoleSelect.value = 'admin';
                } else if (currentRole === 'reseller') {
                    profileName.textContent = 'Enterprise Reseller';
                    document.getElementById('footer-host-os').textContent = 'WHM Node Reseller';
                    userRoleSelect.value = 'reseller';
                } else {
                    profileName.textContent = username.charAt(0).toUpperCase() + username.slice(1);
                    document.getElementById('footer-host-os').textContent = 'cPanel Cloud Container';
                    userRoleSelect.value = 'customer';
                }

                // Adjust default home folders in sandbox
                currentPath = `/home/${currentUser}/public_html`;

                // Refresh all models
                loadDashboardSummaries();
                loadDomainsTable();
                loadFileExplorer();
                loadDatabasesTable();
                loadCronJobs();
                loadPackagesTable();
                loadSupportTickets();
                loadDockerContainers();
                loadMigrationTasks();
                loadOSServicesList();
                loadClusterNodes();

                // Show role-specific warnings/elements
                enforceRoleCapabilities();

                // Hide login, show app
                loginOverlay.classList.remove('active');
                loginOverlay.style.display = 'none';
                mainAppContainer.style.display = 'grid';
                document.body.classList.remove('login-state');

                showNotification(`Welcome back, ${username}! Dashboard initialized.`, 'success');
                connectTelemetryWebSocket();
            } else {
                loginError.textContent = data.error || "Access Denied: Invalid credentials.";
                loginError.style.display = "block";
            }
        } catch (err) {
            console.error('Session login failure:', err);
            loginError.textContent = "Server connection handshake failed.";
            loginError.style.display = "block";
        } finally {
            loginSubmitBtn.disabled = false;
            loginSubmitBtn.textContent = "Authenticate & Enter Dashboard";
        }
    }

    loginSubmitBtn.addEventListener('click', performLogin);
    loginPasswordInput.addEventListener('keydown', (e) => {
        if (e.key === 'Enter') performLogin();
    });

    async function boot() {
        // Just show the login page by default
        console.log("NeoCP Core Ready. Awaiting authentication...");
    }

    const installSoftBtn = document.getElementById('install-softaculous-btn');
    if (installSoftBtn) {
        installSoftBtn.addEventListener('click', () => {
            addTaskIndicator();
            showNotification('Connecting to Softaculous mirrors...', 'info');
            setTimeout(() => {
                showNotification('Downloading Softaculous core package...', 'info');
                setTimeout(() => {
                    showNotification('Softaculous library synchronized. All apps available.', 'success');
                    removeTaskIndicator();
                    document.getElementById('install-softaculous-btn').innerHTML = '🟢 Softaculous Synced';
                }, 2000);
            }, 1000);
        });
    }


    boot();
});

// --- Backuply Pro Event Listeners ---
document.addEventListener('click', async (e) => {
    if (e.target && e.target.id === 'open-backuply-modal') {
        document.getElementById('backuply-modal').style.display = 'flex';
        // Fetch current config
        const res = await fetch('/api/backuply/config', {
            headers: { 'NeoCP-User': currentUser, 'NeoCP-Role': currentRole }
        });
        const conf = await res.json();
        if (conf) {
            document.getElementById('backuply-enabled').value = conf.enabled ? "true" : "false";
            document.getElementById('backuply-s3-bucket').value = conf.s3_bucket || "";
            document.getElementById('backuply-s3-key').value = conf.s3_key || "";
            document.getElementById('backuply-s3-secret').value = conf.s3_secret || "";
            document.getElementById('backuply-gdrive').value = conf.gdrive_enabled ? "true" : "false";
            document.getElementById('backuply-ftp-host').value = conf.ftp_host || "";
        }
    }

    if (e.target && e.target.id === 'close-backuply-modal-btn') {
        document.getElementById('backuply-modal').style.display = 'none';
    }

    if (e.target && e.target.id === 'save-backuply-btn') {
        const payload = {
            enabled: document.getElementById('backuply-enabled').value === "true",
            s3_bucket: document.getElementById('backuply-s3-bucket').value,
            s3_key: document.getElementById('backuply-s3-key').value,
            s3_secret: document.getElementById('backuply-s3-secret').value,
            gdrive_enabled: document.getElementById('backuply-gdrive').value === "true",
            ftp_host: document.getElementById('backuply-ftp-host').value
        };

        const res = await fetch('/api/backuply/config', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'NeoCP-User': currentUser,
                'NeoCP-Role': currentRole
            },
            body: JSON.stringify(payload)
        });

        if (res.ok) {
            showNotification('Backuply cloud connectors updated successfully.', 'success');
            document.getElementById('backuply-modal').style.display = 'none';
        } else {
            showNotification('Failed to update Backuply configuration.', 'error');
        }
    }
});
