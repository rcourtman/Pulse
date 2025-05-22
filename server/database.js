const sqlite3 = require('sqlite3').verbose();
const path = require('path');

// Define the path for the database file. Store it in a 'data' subdirectory.
// Adjust this path if you have a preferred location.
const DB_DIR = path.join(__dirname, '..', 'data');
const DB_PATH = path.join(DB_DIR, 'pulse_history.sqlite3');

// Ensure the data directory exists
const fs = require('fs');
if (!fs.existsSync(DB_DIR)) {
    fs.mkdirSync(DB_DIR, { recursive: true });
}

let db = null;

function initDatabase(callback) {
    db = new sqlite3.Database(DB_PATH, (err) => {
        if (err) {
            console.error('[Database] Error opening database:', err.message);
            return callback(err);
        }
        console.log('[Database] Connected to SQLite database at', DB_PATH);

        db.serialize(() => {
            db.run(
                'CREATE TABLE IF NOT EXISTS metrics_history ('
                + 'id INTEGER PRIMARY KEY AUTOINCREMENT,'
                + 'timestamp INTEGER NOT NULL,'
                + 'guest_unique_id TEXT NOT NULL,'
                + 'metric_name TEXT NOT NULL,'
                + 'value REAL NOT NULL'
                + ')'
            , (err) => {
                if (err) {
                    console.error('[Database] Error creating metrics_history table:', err.message);
                    return callback(err);
                }
                console.log('[Database] metrics_history table ensured.');

                db.run(
                    'CREATE INDEX IF NOT EXISTS idx_metric_query '
                    + 'ON metrics_history (guest_unique_id, metric_name, timestamp)'
                , (indexErr) => {
                    if (indexErr) {
                        console.error('[Database] Error creating index on metrics_history:', indexErr.message);
                    } else {
                        console.log('[Database] Index on metrics_history ensured.');
                    }
                    callback(null);
                });
            });
        });
    });
}

function insertMetricData(timestamp, guestUniqueId, metricName, value, callback) {
    if (!db) {
        return callback(new Error('[Database] Database not initialized.'));
    }
    // +PulseDB Debug Log // Re-enabled for detailed insert logging
    // console.log(`[Database DEBUG] insertMetricData CALLED WITH: ts=${Math.floor(timestamp / 1000)}, id=${guestUniqueId}, metric=${metricName}, val=${value}`);
    const sql = 'INSERT INTO metrics_history (timestamp, guest_unique_id, metric_name, value) VALUES (?, ?, ?, ?)';
    db.run(sql, [Math.floor(timestamp / 1000), guestUniqueId, metricName, value], function(err) {
        if (err) {
            console.error('[Database] Error inserting metric data:', err.message, { timestamp, guestUniqueId, metricName, value });
        }
        if (callback) callback(err, this ? this.lastID : null);
    });
}

// Fetches metrics for a specific guest and metric, ordered by time.
// Default duration: 1 hour. Max results to prevent overload.
function getMetricsForGuest(guestUniqueId, metricName, durationSeconds = 3600, callback) {
    if (!db) {
        return callback(new Error('[Database] Database not initialized.'));
    }
    const sinceTimestamp = Math.floor(Date.now() / 1000) - durationSeconds;
    
    // +PulseDB Enhanced Debug Log
    // console.log(`[Database DEBUG] getMetricsForGuest: PARAMS --- GuestUID: ${guestUniqueId}, Metric: ${metricName}, Duration: ${durationSeconds}s, Calculated sinceTimestamp: ${sinceTimestamp}`);
    
    const sql = 
        'SELECT timestamp, value '
        + 'FROM metrics_history '
        + 'WHERE guest_unique_id = ? AND metric_name = ? AND timestamp >= ? '
        + 'ORDER BY timestamp ASC '
        + 'LIMIT 2000';

    // +PulseDB Enhanced Debug Log
    // console.log(`[Database DEBUG] getMetricsForGuest: EXECUTING SQL --- ${sql.replace(/\s+/g, ' ').trim()} --- PARAMS: [${guestUniqueId}, ${metricName}, ${sinceTimestamp}]`);

    db.all(sql, [guestUniqueId, metricName, sinceTimestamp], (err, rows) => {
        if (err) {
            console.error('[Database] Error fetching metrics:', err.message);
            // +PulseDB Enhanced Debug Log
            // console.log(`[Database DEBUG] getMetricsForGuest: SQL ERROR --- ${err.message} --- For: ${guestUniqueId}, ${metricName}`);
        }
        // +PulseDB Enhanced Debug Log
        const resultCount = rows ? rows.length : 'null (error or no rows)';
        // console.log(`[Database DEBUG] getMetricsForGuest: RESULT --- GuestUID: ${guestUniqueId}, Metric: ${metricName}, Count: ${resultCount}`);
        if (rows && rows.length < 5 && rows.length > 0) { // Log first few results if count is low but not zero
            // console.log(`[Database DEBUG] getMetricsForGuest: Few results sample --- ${JSON.stringify(rows)}`);
        } else if (rows && rows.length === 0) {
             // console.log(`[Database DEBUG] getMetricsForGuest: Query returned 0 rows.`);
        }
        callback(err, rows || []);
    });
}

function pruneOldData(daysToKeep = 7, callback) {
    if (!db) {
        return callback(new Error('[Database] Database not initialized.'));
    }
    const cutoffTimestamp = Math.floor(Date.now() / 1000) - (daysToKeep * 24 * 60 * 60);
    const sql = 'DELETE FROM metrics_history WHERE timestamp < ?';

    db.run(sql, [cutoffTimestamp], function(err) {
        if (err) {
            console.error('[Database] Error pruning old data:', err.message);
            if (callback) callback(err);
            return;
        }
        console.log(`[Database] Pruned ${this.changes} old records (older than ${daysToKeep} days).`);
        if (callback) callback(null, this.changes);
    });
}

// Close the database connection
function closeDatabase(callback) {
    if (db) {
        db.close((err) => {
            if (err) {
                console.error('[Database] Error closing database:', err.message);
                return callback(err);
            }
            console.log('[Database] Database connection closed.');
            db = null;
            callback(null);
        });
    } else {
        callback(null);
    }
}

module.exports = {
    initDatabase,
    insertMetricData,
    getMetricsForGuest,
    pruneOldData,
    closeDatabase
}; 