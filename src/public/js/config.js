const PulseApp = window.PulseApp || {};

PulseApp.config = {
    AVERAGING_WINDOW_SIZE: 5,
    INITIAL_PBS_TASK_LIMIT: 5,
    GRAPH_DOWNSAMPLE_POINTS: 60, // Target points for 1-hour graphs (will be superseded by GRAPH_BUCKET_SECONDS if used)
    GRAPH_BUCKET_SECONDS: 60    // Bucket size in seconds for rigid graph downsampling (e.g., 60 for 1-minute buckets)
}; 