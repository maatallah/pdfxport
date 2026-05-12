(function () {
    console.log("🚀 PDFXport Orchestrator Bridge Active");

    // 1. Inject the XHR hook
    const s = document.createElement("script");
    s.src = chrome.runtime.getURL("inject_xhr.js");
    s.onload = () => {
        // Sync initial state after injection
        chrome.storage.local.get(['harvestingEnabled'], (res) => {
            window.postMessage({ type: "SET_HARVEST_STATE", enabled: res.harvestingEnabled !== false }, "*");
        });
        s.remove();
    };
    (document.head || document.documentElement).appendChild(s);

    // 2. Listen for storage changes and notify the injected script
    chrome.storage.onChanged.addListener((changes) => {
        if (changes.harvestingEnabled) {
            window.postMessage({ type: "SET_HARVEST_STATE", enabled: changes.harvestingEnabled.newValue !== false }, "*");
        }
    });

    // 3. Listen for messages from the injected script
    window.addEventListener("message", function (event) {
        if (event.source !== window) return;
        if (event.data.type === "HARVESTED_ID") {
            const { orderNum, projectId, polygons, token } = event.data;

            // NEW: Check if harvesting is enabled before forwarding to Go
            chrome.storage.local.get(['harvestingEnabled'], function(result) {
                if (result.harvestingEnabled === false) {
                    console.log(`⏸️ Bridge: Harvesting is SUSPENDED. Ignoring ${orderNum}.`);
                    return;
                }

                console.log(`📡 Bridge: Forwarding ${orderNum} (ID: ${projectId}, Polygons: ${polygons.length}) to Go`);

                fetch('http://localhost:8765/add', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    orderNum: orderNum,
                    projectId: projectId,
                    documentId: 38,
                    polygons: polygons,
                    lang: 'fr',
                    token: token
                })
            }).catch(err => console.error("❌ Bridge Send Error:", err));
            }); // End of chrome.storage.local.get
        }
    });
})();