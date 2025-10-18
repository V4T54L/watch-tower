const sseClient = {
    connect: (url, { onData, onStatusChange }) => {
        const eventSource = new EventSource(url);

        eventSource.onopen = () => {
            onStatusChange('connected', 'Live connection established.');
        };

        eventSource.onmessage = (event) => {
            try {
                const data = JSON.parse(event.data);
                onData(data);
            } catch (error) {
                console.error('Failed to parse SSE message:', error);
            }
        };

        eventSource.onerror = () => {
            // The EventSource API automatically handles reconnection.
            // This event fires when the connection is lost and before a reconnection attempt.
            onStatusChange('reconnecting', 'Connection lost. Reconnecting...');
        };

        return eventSource; // Return instance to allow for closing it later if needed
    }
};

