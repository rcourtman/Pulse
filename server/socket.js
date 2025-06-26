
const { Server } = require('socket.io');
const stateManager = require('./state');

function initializeSocket(server) {
    const io = new Server(server, {
        cors: {
            origin: "*",
            methods: ["GET", "POST"]
        }
    });

    function sendCurrentStateToSocket(socket) {
        try {
            const fullCurrentState = stateManager.getState();
            const currentPlaceholderStatus = fullCurrentState.isConfigPlaceholder;

            if (stateManager.hasData()) {
                JSON.stringify(fullCurrentState);
                socket.emit('rawData', fullCurrentState);
            } else {
                console.log('No data available yet, sending initial/loading state.');
                socket.emit('initialState', { loading: true, isConfigPlaceholder: currentPlaceholderStatus });
            }
        } catch (error) {
            console.error('[WebSocket] Error serializing state data:', error.message);
            socket.emit('initialState', { 
                loading: false, 
                error: 'State serialization error',
                isConfigPlaceholder: stateManager.getState().isConfigPlaceholder || false 
            });
        }
    }

    io.on('connection', (socket) => {
        console.log('Client connected');
        sendCurrentStateToSocket(socket);

        socket.on('requestData', async () => {
            console.log('Client requested data');
            try {
                sendCurrentStateToSocket(socket);
            } catch (error) {
                console.error('Error processing requestData event:', error);
            }
        });

        socket.on('disconnect', () => {
            console.log('Client disconnected');
        });
    });

    stateManager.alertManager.on('alert', (alert) => {
        if (io.engine.clientsCount > 0) {
            try {
                const safeAlert = stateManager.alertManager.createSafeAlertForEmit(alert);
                console.log(`[Socket] Emitting new alert: ${safeAlert.id}`);
                io.emit('alert', safeAlert);
            } catch (error) {
                console.error('[Socket] Failed to emit alert:', error);
            }
        }
    });

    stateManager.alertManager.on('alertResolved', (alert) => {
        if (io.engine.clientsCount > 0) {
            try {
                const safeAlert = stateManager.alertManager.createSafeAlertForEmit(alert);
                console.log(`[Socket] Emitting alert resolved: ${safeAlert.id}`);
                io.emit('alertResolved', safeAlert);
            } catch (error) {
                console.error('[Socket] Failed to emit alert resolved:', error);
            }
        }
    });

    stateManager.alertManager.on('alertAcknowledged', (alert) => {
        if (io.engine.clientsCount > 0) {
            try {
                const safeAlert = stateManager.alertManager.createSafeAlertForEmit(alert);
                console.log(`[Socket] Emitting alert acknowledged: ${safeAlert.id}`);
                io.emit('alertAcknowledged', safeAlert);
            } catch (error) {
                console.error('[Socket] Failed to emit alert acknowledged:', error);
            }
        }
    });

    stateManager.alertManager.on('rulesRefreshed', () => {
        if (io.engine.clientsCount > 0) {
            try {
                console.log('[Socket] Emitting rules refreshed event');
                io.emit('alertRulesRefreshed');
            } catch (error) {
                console.error('[Socket] Failed to emit rules refreshed:', error);
            }
        }
    });

    return io;
}

module.exports = { initializeSocket };
