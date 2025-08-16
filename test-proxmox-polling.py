#!/usr/bin/env python3
"""
Test script to monitor how frequently Proxmox API values actually change
"""
import time
import json
import subprocess
from datetime import datetime

def get_node_stats():
    """Get node stats directly from Proxmox API using pvesh"""
    try:
        result = subprocess.run(
            ['ssh', 'root@delly', 'pvesh', 'get', '/nodes', '--output-format', 'json'],
            capture_output=True,
            text=True,
            timeout=5
        )
        if result.returncode == 0:
            data = json.loads(result.stdout)
            # Find delly specifically
            node = None
            for n in data:
                if n.get('node') == 'delly':
                    node = n
                    break
            if node is None:
                return None
            return {
                'cpu': node.get('cpu', 0),
                'mem': node.get('mem', 0),
                'maxmem': node.get('maxmem', 0),
                'disk': node.get('disk', 0),
                'maxdisk': node.get('maxdisk', 0),
                'uptime': node.get('uptime', 0)
            }
    except Exception as e:
        print(f"Error: {e}")
        return None

def main():
    print("Monitoring Proxmox API for value changes...")
    print("Polling every 0.5 seconds to catch any changes")
    print("-" * 80)
    
    last_stats = None
    last_change_time = None
    poll_count = 0
    change_count = 0
    
    # Track when each metric last changed
    last_changes = {}
    
    # Run for 60 seconds
    start_time = time.time()
    duration = 60
    
    while time.time() - start_time < duration:
        poll_count += 1
        current_time = datetime.now().strftime('%H:%M:%S.%f')[:-3]
        stats = get_node_stats()
        
        if stats is None:
            print(f"{current_time} - Failed to get stats")
            time.sleep(0.5)
            continue
        
        if last_stats is None:
            # First poll
            print(f"{current_time} - Initial values:")
            print(f"  CPU: {stats['cpu']:.10f}")
            print(f"  Memory: {stats['mem']} / {stats['maxmem']}")
            print(f"  Disk: {stats['disk']} / {stats['maxdisk']}")
            print(f"  Uptime: {stats['uptime']}")
            for key in stats:
                last_changes[key] = current_time
        else:
            # Check what changed
            changes = []
            for key in stats:
                if stats[key] != last_stats[key]:
                    time_since_last = None
                    if key in last_changes:
                        # Calculate seconds since last change
                        try:
                            prev_time = datetime.strptime(last_changes[key], '%H:%M:%S.%f')
                            curr_time = datetime.strptime(current_time, '%H:%M:%S.%f')
                            delta = (curr_time - prev_time).total_seconds()
                            time_since_last = f"{delta:.1f}s"
                        except:
                            pass
                    
                    if key == 'cpu':
                        changes.append(f"CPU: {last_stats[key]:.10f} -> {stats[key]:.10f} (after {time_since_last})")
                    elif key in ['mem', 'disk']:
                        changes.append(f"{key.upper()}: {last_stats[key]} -> {stats[key]} (after {time_since_last})")
                    elif key == 'uptime':
                        changes.append(f"Uptime: +{stats[key] - last_stats[key]}s (after {time_since_last})")
                    
                    last_changes[key] = current_time
            
            if changes:
                change_count += 1
                print(f"{current_time} - CHANGES DETECTED (poll #{poll_count}):")
                for change in changes:
                    print(f"  {change}")
                last_change_time = current_time
        
        last_stats = stats
        time.sleep(0.5)  # Poll every 500ms to catch any changes
    
    # Summary
    print("\n" + "=" * 80)
    print("SUMMARY:")
    print(f"Total polls: {poll_count}")
    print(f"Total changes detected: {change_count}")
    print(f"Average time between changes: {duration/change_count if change_count > 0 else 0:.1f} seconds")
    print("\nTime between changes for each metric:")
    
    # This is approximate based on change count
    if change_count > 0:
        avg_interval = poll_count / change_count * 0.5
        print(f"Estimated update interval: ~{avg_interval:.1f} seconds")

if __name__ == "__main__":
    main()