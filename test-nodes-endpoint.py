#!/usr/bin/env python3
"""
Test /nodes endpoint to see update frequency
"""
import time
import json
import subprocess
from datetime import datetime

def get_nodes_data():
    """Get all nodes data"""
    try:
        result = subprocess.run(
            ['ssh', 'root@delly', 'pvesh', 'get', '/nodes', '--output-format', 'json'],
            capture_output=True,
            text=True,
            timeout=5
        )
        if result.returncode == 0:
            return json.loads(result.stdout)
    except Exception as e:
        print(f"Error: {e}")
        return None

def main():
    print("Testing /nodes endpoint - tracking delly specifically")
    print("Polling every 1 second for 30 seconds")
    print("-" * 80)
    
    last_cpu = None
    last_mem = None
    cpu_changes = []
    mem_changes = []
    
    for i in range(30):
        current_time = datetime.now().strftime('%H:%M:%S')
        nodes = get_nodes_data()
        
        if nodes is None:
            print(f"{current_time} - Failed to get data")
            time.sleep(1)
            continue
        
        # Find delly
        delly = None
        for node in nodes:
            if node.get('node') == 'delly':
                delly = node
                break
        
        if delly is None:
            print(f"{current_time} - Delly not found")
            time.sleep(1)
            continue
        
        cpu = delly.get('cpu', 0)
        mem = delly.get('mem', 0)
        
        if last_cpu is not None:
            if cpu != last_cpu:
                print(f"{current_time} - CPU changed: {last_cpu:.10f} -> {cpu:.10f}")
                cpu_changes.append(i)
            
            if mem != last_mem:
                delta_mb = (mem - last_mem) / (1024*1024)
                print(f"{current_time} - Mem changed: {delta_mb:+.1f} MB")
                mem_changes.append(i)
        else:
            print(f"{current_time} - Initial: CPU={cpu:.10f}, Mem={mem/(1024*1024*1024):.2f} GB")
        
        last_cpu = cpu
        last_mem = mem
        time.sleep(1)
    
    print(f"\nCPU changes at seconds: {cpu_changes}")
    print(f"Memory changes at seconds: {mem_changes}")
    
    if len(cpu_changes) > 1:
        intervals = [cpu_changes[i+1] - cpu_changes[i] for i in range(len(cpu_changes)-1)]
        print(f"CPU change intervals: {intervals}")
        print(f"Average CPU update interval: {sum(intervals)/len(intervals):.1f} seconds")

if __name__ == "__main__":
    main()