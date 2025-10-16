const ui = {
    elements: {
        ingestRate: document.getElementById('ingest-rate'),
        statusIndicator: document.getElementById('status-indicator'),
        statusText: document.getElementById('status-text'),
        sparkline: document.getElementById('sparkline'),
    },

    updateIngestRate: (rate) => {
        if (ui.elements.ingestRate) {
            ui.elements.ingestRate.textContent = rate.toFixed(2);
        }
    },

    updateStatus: (status, message) => {
        if (ui.elements.statusIndicator && ui.elements.statusText) {
            ui.elements.statusIndicator.className = status; // 'connected', 'reconnecting', 'error'
            ui.elements.statusText.textContent = message;
        }
    },

    drawSparkline: (dataPoints, maxPoints) => {
        if (!ui.elements.sparkline) return;

        const sparkline = ui.elements.sparkline;
        sparkline.innerHTML = ''; // Clear previous bars

        const maxValue = Math.max(...dataPoints, 1); // Avoid division by zero

        // Pad the data with zeros if there are fewer points than maxPoints
        const paddedData = Array(maxPoints - dataPoints.length).fill(0).concat(dataPoints);

        paddedData.forEach(value => {
            const bar = document.createElement('div');
            bar.className = 'sparkline-bar';
            const height = (value / maxValue) * 100;
            bar.style.height = `${Math.max(height, 1)}%`; // Ensure a minimum visible height
            sparkline.appendChild(bar);
        });
    }
};
