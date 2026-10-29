// Dialog and Notification Utility (Modal Alert, Confirm, Toast, and DNS TXT Modal)

let confirmResolve = null;

export function initDialogs() {
    if (document.getElementById('globalModalContainer')) return;

    const modalHTML = `
    <!-- Global Alert / Confirm Modal -->
    <div id="globalModalContainer" class="fixed inset-0 z-[100] hidden overflow-y-auto bg-black/60 backdrop-blur-sm flex items-center justify-center p-4 transition-opacity">
        <div class="bg-light-card dark:bg-dark-card border border-light-border dark:border-dark-border rounded-2xl w-full max-w-md shadow-2xl overflow-hidden flex flex-col transform transition-transform scale-95 duration-150" id="globalModalBox">
            <div class="px-5 py-4 border-b border-light-border dark:border-dark-border flex items-center justify-between bg-light-header dark:bg-dark-header">
                <div class="flex items-center gap-2.5">
                    <div id="globalModalIcon" class="w-8 h-8 rounded-lg flex items-center justify-center text-sm font-bold bg-cyan-500/10 text-cyan-500">
                        <i class="bi bi-info-circle"></i>
                    </div>
                    <h3 id="globalModalTitle" class="text-sm font-bold text-light-text dark:text-white">Pemberitahuan</h3>
                </div>
                <button type="button" onclick="window.closeGlobalModal(false)" class="text-light-muted hover:text-light-text dark:text-dark-muted dark:hover:text-white transition-colors">
                    <i class="bi bi-x-lg text-sm"></i>
                </button>
            </div>
            <div class="p-5 text-sm text-light-text dark:text-slate-300 leading-relaxed space-y-3" id="globalModalBody">
                Pesan
            </div>
            <div class="px-5 py-3 border-t border-light-border dark:border-dark-border flex items-center justify-end gap-2.5 bg-slate-50 dark:bg-dark-header" id="globalModalFooter">
                <button type="button" id="globalModalCancelBtn" onclick="window.closeGlobalModal(false)" class="px-4 py-2 rounded-xl text-xs font-semibold border border-light-border dark:border-dark-border hover:bg-slate-100 dark:hover:bg-dark-border transition-colors hidden">
                    Batal
                </button>
                <button type="button" id="globalModalConfirmBtn" onclick="window.closeGlobalModal(true)" class="px-4 py-2 rounded-xl text-xs font-bold bg-cyan-500 hover:bg-cyan-600 text-white shadow-lg shadow-cyan-500/20 transition-all">
                    Mengerti
                </button>
            </div>
        </div>
    </div>

    <!-- Global Toast Container -->
    <div id="globalToastContainer" class="fixed bottom-5 right-5 z-[110] flex flex-col gap-2.5 pointer-events-none max-w-sm w-full"></div>

    <!-- Dedicated DKIM & DNS Record Modal -->
    <div id="globalDkimModal" class="fixed inset-0 z-[100] hidden overflow-y-auto bg-black/60 backdrop-blur-sm flex items-center justify-center p-4">
        <div class="bg-light-card dark:bg-dark-card border border-light-border dark:border-dark-border rounded-2xl w-full max-w-2xl shadow-2xl overflow-hidden flex flex-col">
            <div class="px-5 py-4 border-b border-light-border dark:border-dark-border flex items-center justify-between bg-light-header dark:bg-dark-header">
                <div class="flex items-center gap-2.5">
                    <div class="w-8 h-8 rounded-lg flex items-center justify-center text-sm font-bold bg-cyan-500/10 text-cyan-500">
                        <i class="bi bi-shield-lock"></i>
                    </div>
                    <div>
                        <h3 class="text-sm font-bold text-light-text dark:text-white">Record DNS TXT (DKIM Key)</h3>
                        <p class="text-[11px] text-light-muted dark:text-dark-muted" id="dkimDomainSubtitle">Domain DNS Verification</p>
                    </div>
                </div>
                <button type="button" onclick="window.closeDkimModal()" class="text-light-muted hover:text-light-text dark:text-dark-muted dark:hover:text-white transition-colors">
                    <i class="bi bi-x-lg text-sm"></i>
                </button>
            </div>
            <div class="p-5 space-y-4 text-xs">
                <p class="text-light-muted dark:text-slate-300">
                    Tambahkan record DNS berikut pada DNS Management domain Anda (misal: Cloudflare, cPanel, atau registrar) agar email ditandatangani secara kriptografis dan tidak masuk spam:
                </p>

                <!-- Field 1: Host / Name -->
                <div class="space-y-1">
                    <div class="flex items-center justify-between">
                        <label class="font-bold text-light-text dark:text-white">Host / Name</label>
                        <button type="button" id="btnCopyDkimHost" class="text-cyan-500 hover:text-cyan-400 font-semibold flex items-center gap-1 transition-colors">
                            <i class="bi bi-copy"></i> Salin Host
                        </button>
                    </div>
                    <div class="p-2.5 rounded-xl bg-slate-100 dark:bg-dark-bg border border-light-border dark:border-dark-border font-mono text-[11px] text-light-text dark:text-cyan-300 break-all select-all" id="dkimHostVal"></div>
                </div>

                <!-- Field 2: Record Type -->
                <div class="space-y-1">
                    <label class="font-bold text-light-text dark:text-white">Record Type</label>
                    <div class="p-2.5 rounded-xl bg-slate-100 dark:bg-dark-bg border border-light-border dark:border-dark-border font-mono text-[11px] font-bold text-light-text dark:text-emerald-400">TXT</div>
                </div>

                <!-- Field 3: TXT Value -->
                <div class="space-y-1">
                    <div class="flex items-center justify-between">
                        <label class="font-bold text-light-text dark:text-white">TXT Value (RSA Public Key)</label>
                        <button type="button" id="btnCopyDkimVal" class="text-cyan-500 hover:text-cyan-400 font-semibold flex items-center gap-1 transition-colors">
                            <i class="bi bi-copy"></i> Salin Nilai TXT
                        </button>
                    </div>
                    <div class="p-3 rounded-xl bg-slate-100 dark:bg-dark-bg border border-light-border dark:border-dark-border font-mono text-[11px] text-light-text dark:text-slate-200 max-h-40 overflow-y-auto break-all select-all leading-relaxed whitespace-pre-wrap" id="dkimTextVal"></div>
                </div>

                <div class="p-3 rounded-xl bg-cyan-500/10 border border-cyan-500/20 text-cyan-600 dark:text-cyan-400 flex items-start gap-2.5">
                    <i class="bi bi-info-circle-fill text-sm mt-0.5"></i>
                    <div>
                        <strong>Catatan:</strong> Propagasi DNS biasanya membutuhkan waktu antara beberapa menit hingga 24 jam tergantung konfigurasi TTL DNS Anda.
                    </div>
                </div>
            </div>
            <div class="px-5 py-3 border-t border-light-border dark:border-dark-border flex items-center justify-end bg-slate-50 dark:bg-dark-header">
                <button type="button" onclick="window.closeDkimModal()" class="px-4 py-2 rounded-xl text-xs font-bold bg-cyan-500 hover:bg-cyan-600 text-white transition-all shadow-lg shadow-cyan-500/20">
                    Selesai
                </button>
            </div>
        </div>
    </div>
    `;

    const div = document.createElement('div');
    div.innerHTML = modalHTML;
    document.body.appendChild(div);
}

export function showToast(message, type = 'info') {
    initDialogs();
    const container = document.getElementById('globalToastContainer');
    if (!container) return;

    const toast = document.createElement('div');
    toast.className = 'pointer-events-auto flex items-center gap-3 px-4 py-3 rounded-xl shadow-xl border text-xs font-semibold transform transition-all duration-200 translate-y-4 opacity-0';

    let icon = 'bi-info-circle-fill';
    if (type === 'success') {
        toast.className += ' bg-emerald-500/95 text-white border-emerald-400 shadow-emerald-500/20';
        icon = 'bi-check-circle-fill';
    } else if (type === 'error' || type === 'danger') {
        toast.className += ' bg-rose-500/95 text-white border-rose-400 shadow-rose-500/20';
        icon = 'bi-exclamation-triangle-fill';
    } else if (type === 'warning') {
        toast.className += ' bg-amber-500/95 text-white border-amber-400 shadow-amber-500/20';
        icon = 'bi-exclamation-circle-fill';
    } else {
        toast.className += ' bg-slate-900/95 dark:bg-dark-card/95 text-white border-light-border dark:border-dark-border shadow-cyan-500/10';
        icon = 'bi-info-circle-fill';
    }

    toast.innerHTML = `
        <i class="bi ${icon} text-base flex-shrink-0"></i>
        <div class="flex-1">${message}</div>
        <button type="button" class="text-white/80 hover:text-white text-sm" onclick="this.parentElement.remove()">
            <i class="bi bi-x-lg"></i>
        </button>
    `;

    container.appendChild(toast);

    requestAnimationFrame(() => {
        toast.classList.remove('translate-y-4', 'opacity-0');
    });

    setTimeout(() => {
        toast.classList.add('opacity-0', 'translate-y-2');
        setTimeout(() => toast.remove(), 200);
    }, 4000);
}

export function showModalAlert(options) {
    initDialogs();
    return new Promise((resolve) => {
        confirmResolve = () => resolve(true);

        const container = document.getElementById('globalModalContainer');
        const box = document.getElementById('globalModalBox');
        const titleEl = document.getElementById('globalModalTitle');
        const bodyEl = document.getElementById('globalModalBody');
        const iconEl = document.getElementById('globalModalIcon');
        const cancelBtn = document.getElementById('globalModalCancelBtn');
        const confirmBtn = document.getElementById('globalModalConfirmBtn');

        const title = typeof options === 'string' ? 'Informasi' : (options.title || 'Informasi');
        const message = typeof options === 'string' ? options : (options.message || '');
        const type = options.type || 'info';
        const confirmText = options.confirmText || 'Mengerti';

        titleEl.textContent = title;
        bodyEl.innerHTML = message.replace(/\n/g, '<br/>');
        confirmBtn.textContent = confirmText;
        cancelBtn.classList.add('hidden');

        // Style Icon & Button
        if (type === 'error' || type === 'danger') {
            iconEl.className = 'w-8 h-8 rounded-lg flex items-center justify-center text-sm font-bold bg-rose-500/10 text-rose-500';
            iconEl.innerHTML = '<i class="bi bi-exclamation-octagon"></i>';
            confirmBtn.className = 'px-4 py-2 rounded-xl text-xs font-bold bg-rose-500 hover:bg-rose-600 text-white shadow-lg shadow-rose-500/20 transition-all';
        } else if (type === 'warning') {
            iconEl.className = 'w-8 h-8 rounded-lg flex items-center justify-center text-sm font-bold bg-amber-500/10 text-amber-500';
            iconEl.innerHTML = '<i class="bi bi-exclamation-triangle"></i>';
            confirmBtn.className = 'px-4 py-2 rounded-xl text-xs font-bold bg-amber-500 hover:bg-amber-600 text-white shadow-lg shadow-amber-500/20 transition-all';
        } else if (type === 'success') {
            iconEl.className = 'w-8 h-8 rounded-lg flex items-center justify-center text-sm font-bold bg-emerald-500/10 text-emerald-500';
            iconEl.innerHTML = '<i class="bi bi-check-circle"></i>';
            confirmBtn.className = 'px-4 py-2 rounded-xl text-xs font-bold bg-emerald-500 hover:bg-emerald-600 text-white shadow-lg shadow-emerald-500/20 transition-all';
        } else {
            iconEl.className = 'w-8 h-8 rounded-lg flex items-center justify-center text-sm font-bold bg-cyan-500/10 text-cyan-500';
            iconEl.innerHTML = '<i class="bi bi-info-circle"></i>';
            confirmBtn.className = 'px-4 py-2 rounded-xl text-xs font-bold bg-cyan-500 hover:bg-cyan-600 text-white shadow-lg shadow-cyan-500/20 transition-all';
        }

        container.classList.remove('hidden');
        requestAnimationFrame(() => {
            box.classList.remove('scale-95');
            box.classList.add('scale-100');
        });
    });
}

export function showModalConfirm(options) {
    initDialogs();
    return new Promise((resolve) => {
        confirmResolve = (val) => resolve(val);

        const container = document.getElementById('globalModalContainer');
        const box = document.getElementById('globalModalBox');
        const titleEl = document.getElementById('globalModalTitle');
        const bodyEl = document.getElementById('globalModalBody');
        const iconEl = document.getElementById('globalModalIcon');
        const cancelBtn = document.getElementById('globalModalCancelBtn');
        const confirmBtn = document.getElementById('globalModalConfirmBtn');

        const title = typeof options === 'string' ? 'Konfirmasi' : (options.title || 'Konfirmasi');
        const message = typeof options === 'string' ? options : (options.message || '');
        const type = options.type || 'warning';
        const confirmText = options.confirmText || 'Ya, Lanjutkan';
        const cancelText = options.cancelText || 'Batal';

        titleEl.textContent = title;
        bodyEl.innerHTML = message.replace(/\n/g, '<br/>');
        confirmBtn.textContent = confirmText;
        cancelBtn.textContent = cancelText;
        cancelBtn.classList.remove('hidden');

        if (type === 'danger' || type === 'error') {
            iconEl.className = 'w-8 h-8 rounded-lg flex items-center justify-center text-sm font-bold bg-rose-500/10 text-rose-500';
            iconEl.innerHTML = '<i class="bi bi-trash3"></i>';
            confirmBtn.className = 'px-4 py-2 rounded-xl text-xs font-bold bg-rose-500 hover:bg-rose-600 text-white shadow-lg shadow-rose-500/20 transition-all';
        } else {
            iconEl.className = 'w-8 h-8 rounded-lg flex items-center justify-center text-sm font-bold bg-amber-500/10 text-amber-500';
            iconEl.innerHTML = '<i class="bi bi-question-circle"></i>';
            confirmBtn.className = 'px-4 py-2 rounded-xl text-xs font-bold bg-cyan-500 hover:bg-cyan-600 text-white shadow-lg shadow-cyan-500/20 transition-all';
        }

        container.classList.remove('hidden');
        requestAnimationFrame(() => {
            box.classList.remove('scale-95');
            box.classList.add('scale-100');
        });
    });
}

export function closeGlobalModal(result) {
    const container = document.getElementById('globalModalContainer');
    const box = document.getElementById('globalModalBox');
    if (box) {
        box.classList.remove('scale-100');
        box.classList.add('scale-95');
    }
    if (container) {
        container.classList.add('hidden');
    }
    if (confirmResolve) {
        confirmResolve(result);
        confirmResolve = null;
    }
}

export function showDkimModal(domainName, selector, publicKey) {
    initDialogs();
    const modal = document.getElementById('globalDkimModal');
    const subtitle = document.getElementById('dkimDomainSubtitle');
    const hostVal = document.getElementById('dkimHostVal');
    const textVal = document.getElementById('dkimTextVal');
    const copyHostBtn = document.getElementById('btnCopyDkimHost');
    const copyValBtn = document.getElementById('btnCopyDkimVal');

    const sel = selector || 'mail';
    const hostRecord = `${sel}._domainkey.${domainName}`;
    const txtRecord = publicKey || `v=DKIM1; k=rsa; p=MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA...`;

    subtitle.textContent = `Konfigurasi Verifikasi Email untuk ${domainName}`;
    hostVal.textContent = hostRecord;
    textVal.textContent = txtRecord;

    copyHostBtn.onclick = () => {
        navigator.clipboard.writeText(hostRecord).then(() => {
            showToast('Host DKIM berhasil disalin ke clipboard!', 'success');
        });
    };

    copyValBtn.onclick = () => {
        navigator.clipboard.writeText(txtRecord).then(() => {
            showToast('Nilai TXT DKIM berhasil disalin ke clipboard!', 'success');
        });
    };

    modal.classList.remove('hidden');
}

export function closeDkimModal() {
    const modal = document.getElementById('globalDkimModal');
    if (modal) modal.classList.add('hidden');
}

// Bind to window for global access
window.showToast = showToast;
window.showModalAlert = showModalAlert;
window.showModalConfirm = showModalConfirm;
window.closeGlobalModal = closeGlobalModal;
window.showDkimModal = showDkimModal;
window.closeDkimModal = closeDkimModal;
