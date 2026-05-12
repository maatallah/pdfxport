document.addEventListener('DOMContentLoaded', function() {
    const harvestSwitch = document.getElementById('harvestSwitch');
    const statusDot = document.getElementById('statusDot');
    const statusText = document.getElementById('statusText');
    const bufferCount = document.getElementById('bufferCount');
    const btnHarvest = document.getElementById('btnHarvest');
    const btnClear = document.getElementById('btnClear');

    // 1. Health Check for Go Server
    function checkHealth() {
        fetch('http://localhost:8765/health')
            .then(r => r.text())
            .then(text => {
                if (text === "OK") {
                    statusDot.className = "status-dot online";
                    statusText.innerText = "Serveur en ligne";
                    btnHarvest.disabled = false;
                }
            })
            .catch(() => {
                statusDot.className = "status-dot offline";
                statusText.innerText = "Serveur hors ligne";
                btnHarvest.disabled = true;
            });
    }

    // 2. Update Buffer Counter
    function updateBufferUI() {
        chrome.storage.local.get(['stagingBuffer'], function(result) {
            const buffer = result.stagingBuffer || {};
            const count = Object.keys(buffer).length;
            bufferCount.innerText = `${count} commande(s) prête(s)`;
            btnHarvest.disabled = count === 0 || statusDot.classList.contains('offline');
        });
    }

    // Initial load
    chrome.storage.local.get(['harvestingEnabled'], function(result) {
        harvestSwitch.checked = result.harvestingEnabled !== false;
    });

    checkHealth();
    updateBufferUI();
    setInterval(checkHealth, 3000); // Check every 3s

    // NEW: Listen for storage changes to update counter in real-time
    chrome.storage.onChanged.addListener((changes) => {
        if (changes.stagingBuffer) {
            updateBufferUI();
        }
    });

    // Listeners
    harvestSwitch.addEventListener('change', function() {
        chrome.storage.local.set({ harvestingEnabled: harvestSwitch.checked });
    });

    btnClear.addEventListener('click', function() {
        if (confirm("Voulez-vous vraiment vider la file d'attente ?")) {
            chrome.storage.local.set({ stagingBuffer: {} }, updateBufferUI);
        }
    });

    btnHarvest.addEventListener('click', function() {
        // We will implement the trigger logic in Step 4
        chrome.tabs.query({ active: true, currentWindow: true }, function(tabs) {
            chrome.tabs.sendMessage(tabs[0].id, { type: "TRIGGER_HARVEST" });
        });
    });
});
