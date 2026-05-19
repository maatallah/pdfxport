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

            // Debounce the storage set by 100ms to avoid Chrome MAX_WRITE_OPERATIONS_PER_MINUTE rate limiting
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
        let sentCount = 0;
        let failCount = 0;

        // Process sequentially with a tiny delay to avoid browser socket exhaustion and SQLite database write locks
        for (const orderNum of selectedOrders) {
            const data = bufferData[orderNum];

            if (data) {
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
                
                // Sleep 5ms to give Go SQLite server breathing room
                await new Promise(r => setTimeout(r, 5));
            } else {
                console.warn(`⚠️ Épi manquant pour ${orderNum} (Pas encore capturé)`);
                failCount++;
            }
        }

        if (sentCount > 0) {
            if (failCount > 0) {
                alert(`✅ ${sentCount} épi(s) envoyé(s) à la grange.\n⚠️ ${failCount} épi(s) ont échoué ou étaient manquants !`);
            } else {
                alert(`✅ Tous les ${sentCount} épis ont été envoyés à la grange avec succès !`);
            }
        } else {
            alert("⚠️ Aucun épi n'a pu être envoyé à la grange.");
        }
    }
})();
