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

    // 3. The Staging Engine (Mise au Grenier)
    window.addEventListener("message", function (event) {
        if (event.source !== window) return;
        if (event.data.type === "HARVESTED_ID") {
            const { orderNum, projectId, polygons, token } = event.data;

            chrome.storage.local.get(['stagingBuffer'], function(result) {
                const buffer = result.stagingBuffer || {};
                buffer[orderNum] = { projectId, polygons, token, timestamp: Date.now() };
                
                chrome.storage.local.set({ stagingBuffer: buffer }, () => {
                    console.log(`🌾 Épi capturé: ${orderNum} (${polygons.length} polygones)`);
                });
            });
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

        selectedOrders.forEach(orderNum => {
            const data = bufferData[orderNum];

            if (data) {
                fetch('http://localhost:8765/add', {
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
                })
                .then(() => {
                    console.log(`✅ Mis au grenier: ${orderNum}`);
                })
                .catch(err => console.error(`❌ Échec de la moisson pour ${orderNum}:`, err));
                
                sentCount++;
            } else {
                console.warn(`⚠️ Épi manquant pour ${orderNum} (Pas encore capturé)`);
            }
        });

        if (sentCount > 0) {
            alert(`✅ ${sentCount} épi(s) envoyé(s) au grenier !`);
        } else {
            alert("⚠️ Aucun épi prêt pour la moisson.");
        }
    }
})();