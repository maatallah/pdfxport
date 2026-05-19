(function () {
    console.log("🌾 Pont Moissonneuse PDFXport Actif");

    // 1. Inject the XHR hook
    const s = document.createElement("script");
    s.src = chrome.runtime.getURL("inject_xhr.js");
    s.onload = () => {
        chrome.storage.local.get(['harvestingEnabled'], (res) => {
            window.postMessage({ type: "SET_HARVEST_STATE", enabled: res.harvestingEnabled !== false }, "*");
        });
        s.remove();
    };
    (document.head || document.documentElement).appendChild(s);

    // 2. State Sync
    chrome.storage.onChanged.addListener((changes) => {
        if (changes.harvestingEnabled) {
            window.postMessage({ type: "SET_HARVEST_STATE", enabled: changes.harvestingEnabled.newValue !== false }, "*");
        }
    });

    // 3. The Staging Engine (Mise en Grange) — debounced and race-free
    let stagingBuffer = {};
    chrome.storage.local.get(['stagingBuffer'], (res) => {
        stagingBuffer = res.stagingBuffer || {};
    });

    let saveTimeout = null;
    window.addEventListener("message", function (event) {
        if (event.source !== window) return;
        if (event.data.type === "HARVESTED_ID") {
            const { orderNum, projectId, polygons, token } = event.data;

            stagingBuffer[orderNum] = { projectId, polygons, token, timestamp: Date.now() };
            console.log(`🌾 Épi capturé: ${orderNum} (${polygons.length} polygones)`);

            // Debounce storage writes by 100ms to stay well below Chrome's write rate-limits
            if (saveTimeout) clearTimeout(saveTimeout);
            saveTimeout = setTimeout(() => {
                chrome.storage.local.set({ stagingBuffer }, () => {
                    console.log("💾 Grange synchronisée avec succès !");
                });
            }, 100);
        }
    });

    // 4. Trigger
    chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
        if (message.type === "TRIGGER_HARVEST") {
            handleHarvestTrigger();
        }
    });

    async function handleHarvestTrigger() {
        const rows = document.querySelectorAll('tbody tr[role="row"]');
        const selectedOrders = [];

        rows.forEach(row => {
            const checkbox = row.querySelector('mat-checkbox.mat-checkbox-checked');
            if (checkbox) {
                const projectCell = row.querySelector('.cdk-column-projectNumber');
                if (projectCell) {
                    selectedOrders.push(projectCell.innerText.trim());
                }
            }
        });
        
        if (selectedOrders.length === 0) {
            alert("❌ Aucun épi sélectionné pour la moisson !");
            return;
        }

        const bufferData = await new Promise(r => chrome.storage.local.get(['stagingBuffer'], res => r(res.stagingBuffer || {})));
        
        console.log(`🚜 Moisson lancée pour ${selectedOrders.length} épis...`);

        // Create or show the beautiful, premium glassmorphic HUD
        let hud = document.getElementById('pdfxport-harvest-hud');
        if (!hud) {
            hud = document.createElement('div');
            hud.id = 'pdfxport-harvest-hud';
            hud.style.cssText = `
                position: fixed;
                bottom: 25px;
                right: 25px;
                z-index: 9999999;
                background: rgba(18, 24, 38, 0.95);
                color: #ffffff;
                padding: 18px;
                border-radius: 16px;
                box-shadow: 0 12px 40px rgba(0, 0, 0, 0.6);
                border: 1px solid rgba(255, 255, 255, 0.12);
                font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
                min-width: 320px;
                max-width: 360px;
                backdrop-filter: blur(12px);
                -webkit-backdrop-filter: blur(12px);
                transition: all 0.3s cubic-bezier(0.16, 1, 0.3, 1);
                opacity: 0;
                transform: translateY(10px);
            `;
            document.body.appendChild(hud);
            // Force redraw for smooth slide-up animation
            hud.offsetHeight;
        }
        
        hud.style.opacity = '1';
        hud.style.transform = 'translateY(0)';
        
        hud.innerHTML = `
            <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px;">
                <span style="font-weight: 700; font-size: 14px; color: #4ade80; display: flex; align-items: center; gap: 8px; letter-spacing: 0.5px;">
                    🌾 MOISSONNEUSE ACTIVE
                </span>
                <button id="close-hud-btn" style="background: rgba(255,255,255,0.08); border: none; color: #a0aec0; cursor: pointer; font-size: 14px; width: 24px; height: 24px; border-radius: 50%; display: flex; align-items: center; justify-content: center; transition: background 0.2s;">&times;</button>
            </div>
            <div id="hud-status" style="font-size: 13px; color: #e2e8f0; margin-bottom: 10px; font-weight: 500;">
                🚜 Initialisation du convoi...
            </div>
            <div style="background: rgba(255,255,255,0.08); border-radius: 8px; height: 10px; overflow: hidden; margin-bottom: 10px; border: 1px solid rgba(255,255,255,0.05);">
                <div id="hud-bar" style="background: linear-gradient(90deg, #4ade80, #10b981); width: 0%; height: 100%; border-radius: 8px; transition: width 0.25s cubic-bezier(0.4, 0, 0.2, 1);"></div>
            </div>
            <div id="hud-counter" style="display: flex; justify-content: space-between; font-size: 12px; color: #94a3b8; font-weight: 500;">
                <span>0 / ${selectedOrders.length} récoltés</span>
                <span id="hud-percent" style="font-weight: 700; color: #4ade80;">0%</span>
            </div>
        `;

        const closeBtn = document.getElementById('close-hud-btn');
        closeBtn.onclick = () => {
            hud.style.opacity = '0';
            hud.style.transform = 'translateY(10px)';
            setTimeout(() => { hud.remove(); }, 300);
        };
        closeBtn.onmouseenter = () => { closeBtn.style.background = 'rgba(255,255,255,0.15)'; closeBtn.style.color = '#fff'; };
        closeBtn.onmouseleave = () => { closeBtn.style.background = 'rgba(255,255,255,0.08)'; closeBtn.style.color = '#a0aec0'; };

        let sentCount = 0;
        let failCount = 0;

        // Process sequentially with a tiny delay to avoid browser socket exhaustion and SQLite database write locks
        for (const orderNum of selectedOrders) {
            const data = bufferData[orderNum];

            if (data) {
                // Update active status before download begins
                const hudStatus = document.getElementById('hud-status');
                if (hudStatus) {
                    hudStatus.innerHTML = `🚜 En cours : <span style="color:#60a5fa; font-weight:700;">${orderNum}</span>`;
                }

                try {
                    const response = await fetch('http://localhost:8765/add', {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({
                            orderNum: orderNum,
                            projectId: data.projectId,
                            documentId: 38,
                            polygons: data.polygons,
                            lang: 'fr',
                            token: data.token
                        })
                    });
                    
                    if (response.ok) {
                        console.log(`✅ Mis en grange: ${orderNum}`);
                        sentCount++;
                    } else {
                        console.error(`❌ Échec de la moisson pour ${orderNum}: Status ${response.status}`);
                        failCount++;
                    }
                } catch (err) {
                    console.error(`❌ Échec réseau pour ${orderNum}:`, err);
                    failCount++;
                }
                
                // Update HUD in real time
                const processed = sentCount + failCount;
                const percent = Math.round((processed / selectedOrders.length) * 100);
                
                const hudBar = document.getElementById('hud-bar');
                const hudCounter = document.getElementById('hud-counter');
                const hudPercent = document.getElementById('hud-percent');
                
                if (hudBar) hudBar.style.width = `${percent}%`;
                if (hudCounter) {
                    hudCounter.innerHTML = `<span>${processed} / ${selectedOrders.length} traités (${sentCount} en grange)</span>`;
                }
                if (hudPercent) hudPercent.innerText = `${percent}%`;

                // Sleep 5ms to give Go SQLite server breathing room
                await new Promise(r => setTimeout(r, 5));
            } else {
                console.warn(`⚠️ Épi manquant pour ${orderNum} (Pas encore capturé)`);
                failCount++;
                
                const processed = sentCount + failCount;
                const percent = Math.round((processed / selectedOrders.length) * 100);
                const hudBar = document.getElementById('hud-bar');
                const hudCounter = document.getElementById('hud-counter');
                const hudPercent = document.getElementById('hud-percent');
                
                if (hudBar) hudBar.style.width = `${percent}%`;
                if (hudCounter) {
                    hudCounter.innerHTML = `<span>${processed} / ${selectedOrders.length} traités (${sentCount} en grange)</span>`;
                }
                if (hudPercent) hudPercent.innerText = `${percent}%`;
            }
        }

        // Finalize HUD state
        const hudStatus = document.getElementById('hud-status');
        if (hudStatus) {
            if (sentCount === selectedOrders.length) {
                hudStatus.innerHTML = `🎉 <span style="color:#4ade80; font-weight:700;">Moisson réussie avec succès !</span>`;
            } else {
                hudStatus.innerHTML = `⚠️ <span style="color:#fbbf24; font-weight:700;">Terminé avec ${failCount} échec(s).</span>`;
            }
            
            // Auto fade out after 8 seconds unless closed manually
            setTimeout(() => {
                const activeHud = document.getElementById('pdfxport-harvest-hud');
                if (activeHud) {
                    activeHud.style.opacity = '0';
                    activeHud.style.transform = 'translateY(10px)';
                    setTimeout(() => { activeHud.remove(); }, 300);
                }
            }, 8000);
        }
    }
})();
