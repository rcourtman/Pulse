
import sys
import os

def parse_coverage(filename):
    if not os.path.exists(filename):
        print(f"File {filename} not found")
        return

    package_stmts = {}
    package_covered = {}

    with open(filename, 'r') as f:
        lines = f.readlines()

    current_mode = ""
    for line in lines:
        if line.startswith("mode:"):
            current_mode = line.split()[1]
            continue
        
        parts = line.strip().split(':')
        if len(parts) != 2:
            continue
        
        file_path = parts[0]
        # Package is directory of file_path
        package_name = os.path.dirname(file_path)
        
        metrics = parts[1].split()
        if len(metrics) != 3:
            continue
            
        # start_end = metrics[0]
        num_stmts = int(metrics[1])
        count = int(metrics[2])
        
        package_stmts[package_name] = package_stmts.get(package_name, 0) + num_stmts
        if count > 0:
            package_covered[package_name] = package_covered.get(package_name, 0) + num_stmts

    results = []
    for pkg, total in package_stmts.items():
        covered = package_covered.get(pkg, 0)
        percent = (covered / total) * 100 if total > 0 else 0
        results.append((pkg, percent, covered, total))

    # Sort by percentage (ascending)
    results.sort(key=lambda x: x[1])

    print("Package Coverage Report (Bottom 20):")
    for pkg, pct, cov, tot in results[:20]:
        print(f"{pct:6.2f}% ({cov}/{tot}) {pkg}")

if __name__ == "__main__":
    parse_coverage("coverage.out")
