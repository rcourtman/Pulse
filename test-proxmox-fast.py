#!/usr/bin/env python3
"""
Test with specific node endpoint instead of /nodes
"""
import time
import json
import subprocess
from datetime import datetime

def get_node_stats_specific():
    """Get delly stats from specific node endpoint"""
    try:
        # Use the specific node endpoint
        result = subprocess.run(
            ['ssh', 'root@delly', 'pvesh', 'get', '/nodes/delly/status', '--output-format', 'json'],
            capture_output=True,
            text=True,
            timeout=5
        )
        if result.returncode == 0:
            node = json.loads(result.stdout)
            return {
                'cpu': node.get('cpu', 0),
                'wait': node.get('wait', 0),
                'load': node.get('loadavg', [0])[0] if 'loadavg' in node else 0,
                'mem_used': node.get('memory', {}).get('used', 0),
                'mem_total': node.get('memory', {}).get('total', 0),
                'uptime': node.get('uptime', 0)
            }
    except Exception as e:
        print(f"Error: {e}")
        return None

def main():
    print("Testing /nodes/delly/status endpoint specifically")
    print("Polling every 1 second for 30 seconds")
    print("-" * 80)
    
    last_stats = None
    changes_at = []
    
    for i in range(30):
        current_time = datetime.now().strftime('%H:%M:%S')
        stats = get_node_stats_specific()
        
        if stats is None:
            print(f"{current_time} - Failed to get stats")
            time.sleep(1)
            continue
        
        if last_stats is not None:
            # Check if CPU changed
            if stats['cpu'] != last_stats['cpu']:
                delta = stats['cpu'] - last_stats['cpu']
                print(f"{current_time} - CPU changed: {last_stats['cpu']:.10f} -> {stats['cpu']:.10f} (delta: {delta:+.10f})")
                changes_at.append(i)
            
            # Check memory
            if stats['mem_used'] != last_stats['mem_used']:
                delta_mb = (stats['mem_used'] - last_stats['mem_used']) / (1024*1024)
                print(f"{current_time} - Memory changed: {delta_mb:+.1f} MB")
        else:
            print(f"{current_time} - Initial CPU: {stats['cpu']:.10f}, Mem: {stats['mem_used']/(1024*1024*1024):.2f} GB")
        
        last_stats = stats
        time.sleep(1)
    
    if len(changes_at) > 1:
        intervals = [changes_at[i+1] - changes_at[i] for i in range(len(changes_at)-1)]
        avg_interval = sum(intervals) / len(intervals) if intervals else 0
        print(f"\nChanges detected at seconds: {changes_at}")
        print(f"Intervals between changes: {intervals}")
        print(f"Average interval: {avg_interval:.1f} seconds")
    else:
        print(f"\nOnly {len(changes_at)} changes detected in 30 seconds")

if __name__ == "__main__":
    main()