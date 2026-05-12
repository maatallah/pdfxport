document.addEventListener('DOMContentLoaded', function() {
    const harvestSwitch = document.getElementById('harvestSwitch');

    // Load current state
    chrome.storage.local.get(['harvestingEnabled'], function(result) {
        // Default to true if not set
        const isEnabled = result.harvestingEnabled !== false;
        harvestSwitch.checked = isEnabled;
    });

    // Save state on change
    harvestSwitch.addEventListener('change', function() {
        chrome.storage.local.set({ harvestingEnabled: harvestSwitch.checked });
    });
});
