(function() {
    const MAX_SPARKLINE_POINTS = 60; // Show last 60 seconds of data
    let dataPoints = [];

    const handleNewData = (data) => {
        const rate = data.Rate || 0;
        
        // Update data for sparkline
        dataPoints.push(rate);
        if (dataPoints.length > MAX_SPARKLINE_POINTS) {
            dataPoints.shift(); // Keep the array size fixed
        }

        // Update UI
        ui.updateIngestRate(rate);
        ui.drawSparkline(dataPoints, MAX_SPARKLINE_POINTS);
    };

    const handleStatusChange = (status, message) => {
        ui.updateStatus(status, message);
    };

    document.addEventListener('DOMContentLoaded', () => {
        // Initialize UI
        ui.updateStatus('reconnecting', 'Initializing...');
        ui.drawSparkline(dataPoints, MAX_SPARKLINE_POINTS);

        // Connect to SSE endpoint
        sseClient.connect('/events', {
            onData: handleNewData,
            onStatusChange: handleStatusChange,
        });
    });
})();

