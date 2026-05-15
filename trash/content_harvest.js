(function() {
    console.log("🚀 PDFXport Harvester Initializing...");

    let harvestInterval = null;

    function startHarvesting() {
        if (harvestInterval) return;
        console.log("🚀 PDFXport Harvester ACTIVE on Body");
        harvestInterval = setInterval(harvest, 2000);
        harvest();
    }

    // Wait for body to exist before starting
    if (document.body) {
        startHarvesting();
    } else {
        const observer = new MutationObserver(() => {
            if (document.body) {
                observer.disconnect();
                startHarvesting();
            }
        });
        observer.observe(document.documentElement, { childList: true });
    }

    const seenOrders = new Set();
    let failureCount = 0;
    let isStopped = false;

    function harvest() {
        if (isStopped) return;
        
        if (failureCount > 100) {
            console.error("🛑 [PANIC] Too many failures (>100). Stopping Harvester.");
            isStopped = true;
            alert("PDFXport: Harvester stopped due to too many row detection failures. Check console.");
            return;
        }
        const pattern = /^[A-Z0-9]{4}\.[A-Z0-9]{5}$/;
        
        const allElements = document.querySelectorAll('div, span, td, a');
        let foundThisTurn = 0;

        console.log(`🔍 [DEBUG] Scanning ${allElements.length} elements...`);

        allElements.forEach(el => {
            const text = el.innerText ? el.innerText.trim() : "";
            
            if (!pattern.test(text)) return;
            
            const orderNum = text;
            if (seenOrders.has(orderNum)) return;

            console.log(`🎯 [MATCH] Found Order Num: ${orderNum}`);

            // Ensure we are looking at the deepest matching element
            if (el.querySelector('*')) {
                const childMatches = Array.from(el.children).some(child => pattern.test(child.innerText || ""));
                if (childMatches) return;
            }

            const row = el.closest('tr') || el.closest('mat-row') || el.closest('.datatable-body-row');
            if (!row) {
                console.warn(`  ⚠️ Row not found for ${orderNum}`);
                return;
            }

            // FUZZY ID SEARCH: Search row HTML for any 5-8 digit number
            // (Most project IDs are in this range)
            const rowHTML = row.innerHTML;
            const idMatches = rowHTML.match(/\b\d{5,8}\b/g) || [];
            
            // Filter out any numbers that are already part of the order number
            projectId = idMatches.find(id => !orderNum.includes(id));

            if (!projectId) {
                console.warn(`  ⚠️ No ID found in row HTML for ${orderNum}.`);
                failureCount++;
                return;
            }

            seenOrders.add(orderNum);
            foundThisTurn++;
            console.log(`🚀 [SENDING] ${orderNum} -> Project ${projectId}`);

            fetch('http://localhost:8765/add', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    orderNum: orderNum,
                    projectId: parseInt(projectId),
                    documentId: 38,
                    polygons: [],
                    lang: 'fr'
                })
            }).catch(err => console.error("❌ Link Error:", err));
        });

        if (foundThisTurn > 0) {
            console.log(`✅ Success: Sent ${foundThisTurn} orders to Go.`);
        }
    }

    // Run every 3 seconds to catch new rows when paging
    setInterval(harvest, 3000);
    harvest();

})();
