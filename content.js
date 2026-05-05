(function() {
    console.log(">> CPT DECOLOOP INTERCEPTOR v2.5 ACTIVE <<");

    const _nativeFetch = window.fetch.bind(window);

    // ======================================================
    // STATE
    // ======================================================
    let pdfIntercepted = false;

    function markIntercept() {
        pdfIntercepted = true;
    }

    // ======================================================
    // LOGGING
    // ======================================================
    function remoteLog(message) {
        const timestamp = new Date().toISOString().replace('T', ' ').substring(0, 19);
        const fullMsg = `[${timestamp}] ${message}`;
        navigator.sendBeacon('http://localhost:8765/log', fullMsg);
    }

    async function sendToProcessor(blob) {
        try {
            const res = await _nativeFetch('http://localhost:8765/upload', {
                method: 'POST',
                body: blob
            });
            if (res.ok) {
                remoteLog("[OK] PDF pushed to Go Server successfully!");
            }
        } catch (err) {
            remoteLog(`[ERROR] Failed to talk to Go Server: ${err.message}`);
        }
    }

    remoteLog('[INIT] v2.5 loaded on ' + window.location.href);

    // ======================================================
    // UI CLEANUP
    // ======================================================
    function clearSpinner() {
        try {
            const targets = [
                'sds-production-print-dialog',
                '.cdk-overlay-container',
                'mat-dialog-container',
                '.cdk-overlay-backdrop'
            ];

            targets.forEach(selector => {
                const elements = document.querySelectorAll(selector);
                elements.forEach(el => {
                    if (selector.includes('dialog') || (el.innerText && el.innerText.includes('Génération'))) {
                        el.style.display = 'none';
                        setTimeout(() => el.remove(), 50);
                    }
                });
            });
        } catch (e) {
            remoteLog(`[ERROR] clearSpinner: ${e.message}`);
        }
    }

    // ======================================================
    // 🔴 HARD BLOCK PRINT (GLOBAL)
    // ======================================================
    const blockPrint = () => {
        remoteLog("[BLOCKED] window.print()");
        clearSpinner();
    };

    window.print = blockPrint;

    // Prevent reassignment (important)
    Object.defineProperty(window, 'print', {
        configurable: false,
        writable: false,
        value: blockPrint
    });

    // Watchdog (in case site re-injects print)
    setInterval(() => {
        window.print = blockPrint;
    }, 500);

    // ======================================================
    // BLOCK beforeprint EVENT
    // ======================================================
    window.addEventListener('beforeprint', (e) => {
        e.preventDefault();
        e.stopImmediatePropagation();
        remoteLog("[BLOCKED] beforeprint event");
    });

    // ======================================================
    // BLOCK matchMedia('print')
    // ======================================================
    const origMatchMedia = window.matchMedia;
    window.matchMedia = function(query) {
        if (query === 'print') {
            return {
                matches: false,
                media: 'print',
                onchange: null,
                addListener: () => {},
                removeListener: () => {},
                addEventListener: () => {},
                removeEventListener: () => {}
            };
        }
        return origMatchMedia.apply(this, arguments);
    };

    // ======================================================
    // BLOCK BLOB PDF PREVIEW
    // ======================================================
    const origCreateObjectURL = URL.createObjectURL;
    URL.createObjectURL = function(blob) {
        if (blob && blob.type === 'application/pdf') {
            remoteLog("[BLOCKED] Blob URL for PDF");
            return "about:blank";
        }
        return origCreateObjectURL.apply(this, arguments);
    };

    // ======================================================
    // BLOCK window.open FOR PDFs
    // ======================================================
    const oldWindowOpen = window.open;
    window.open = function() {
        remoteLog("[BLOCKED] window.open()");
        clearSpinner();
        return null;
    };

    // ======================================================
    // BLOCK iframe PDF LOAD
    // ======================================================
    const iframeSrcDescriptor = Object.getOwnPropertyDescriptor(HTMLIFrameElement.prototype, 'src');
    Object.defineProperty(HTMLIFrameElement.prototype, 'src', {
        set: function(val) {
            if (val && (val.includes('blob:') || val.includes('.pdf'))) {
                remoteLog("[BLOCKED] iframe src PDF");
                clearSpinner();
                return;
            }
            if (iframeSrcDescriptor && iframeSrcDescriptor.set) {
                iframeSrcDescriptor.set.call(this, val);
            }
        }
    });

    // Block iframe.print()
    const origContentWindow = Object.getOwnPropertyDescriptor(HTMLIFrameElement.prototype, 'contentWindow');
    if (origContentWindow && origContentWindow.get) {
        Object.defineProperty(HTMLIFrameElement.prototype, 'contentWindow', {
            get: function() {
                const win = origContentWindow.get.call(this);
                if (win) {
                    try {
                        win.print = blockPrint;
                    } catch(e) {}
                }
                return win;
            }
        });
    }

    // ======================================================
    // BLOCK anchor click (PDF)
    // ======================================================
    const oldAnchorClick = HTMLAnchorElement.prototype.click;
    HTMLAnchorElement.prototype.click = function() {
        if (this.href && (this.href.includes('blob:') || this.href.includes('.pdf'))) {
            remoteLog("[BLOCKED] anchor PDF click");
            clearSpinner();
            return;
        }
        return oldAnchorClick.apply(this, arguments);
    };

    // ======================================================
    // XHR INTERCEPTION
    // ======================================================
    const oldOpen = XMLHttpRequest.prototype.open;
    const oldSend = XMLHttpRequest.prototype.send;
    const oldGetHeader = XMLHttpRequest.prototype.getResponseHeader;
    const oldGetAllHeaders = XMLHttpRequest.prototype.getAllResponseHeaders;

    XMLHttpRequest.prototype.open = function(method, url) {
        this._url = url;
        this._isPdfRequest = url.toLowerCase().includes('generatepdfdocument');
        return oldOpen.apply(this, arguments);
    };

    XMLHttpRequest.prototype.getResponseHeader = function(header) {
        if (this._isPdfRequest) {
            const h = header.toLowerCase();
            if (h === 'content-type') return 'application/octet-stream';
            if (h === 'content-disposition') return null;
        }
        return oldGetHeader.apply(this, arguments);
    };

    XMLHttpRequest.prototype.getAllResponseHeaders = function() {
        let headers = oldGetAllHeaders.apply(this, arguments);
        if (this._isPdfRequest) {
            headers = headers.replace(/content-type: application\/pdf/gi, 'Content-Type: application/octet-stream');
            headers = headers.replace(/content-disposition: .+\r\n/gi, '');
        }
        return headers;
    };

    XMLHttpRequest.prototype.send = function() {
        if (this._isPdfRequest) {
            this.responseType = 'blob';
        }

        this.addEventListener('load', async function() {
            try {
                const realCt = oldGetHeader.call(this, 'Content-Type') || "";
                if (realCt.toLowerCase().includes('application/pdf') || this._isPdfRequest) {
                    const blob = this.response;
                    if (blob instanceof Blob && blob.size > 0) {
                        markIntercept();
                        remoteLog(`🚀 PDF Captured (${blob.size} bytes)`);

                        await sendToProcessor(blob);
                        clearSpinner();
                    }
                }
            } catch (err) {
                remoteLog(`[CRASH] XHR Load: ${err.message}`);
            }
        });

        return oldSend.apply(this, arguments);
    };

    // ======================================================
    // FETCH INTERCEPTION
    // ======================================================
    const oldFetch = window.fetch;
    window.fetch = async function(...args) {
        let reqUrl = (typeof args[0] === 'string') ? args[0] : (args[0]?.url || "");

        const response = await oldFetch.apply(this, args);
        const ct = response.headers.get('Content-Type') || "";

        if (ct.toLowerCase().includes('application/pdf') || reqUrl.toLowerCase().includes('generatepdfdocument')) {
            try {
                const blob = await response.clone().blob();
                if (blob && blob.size > 0) {
                    markIntercept();
                    remoteLog(`🚀 PDF Captured via FETCH (${blob.size} bytes)`);

                    sendToProcessor(blob);
                    clearSpinner();
                }
            } catch (e) {}
        }

        return response;
    };

})();