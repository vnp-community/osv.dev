import os
import re
from collections import defaultdict

root_dir = '/Users/binhnt/Lab/sec/cve/osv.dev'
dirs_to_scan = ['apps/osv', 'services']
output_file = os.path.join(root_dir, 'specs', 'backend_api_specs.md')

# Regex to match chi/mux route definitions like: r.Get("/path", handler)
# We want to capture the HTTP method and the path.
# Added check to ensure the path starts with '/' to filter out r.Header.Get("...") or r.URL.Query().Get("...")
route_pattern = re.compile(r'\.(Get|Post|Put|Delete|Patch)\(\s*"(/[^"]*)"')

services_routes = defaultdict(list)

def scan_directory(base_dir):
    for root, _, files in os.walk(base_dir):
        # Determine service name based on directory structure
        rel_path = os.path.relpath(root, root_dir)
        parts = rel_path.split(os.sep)
        
        service_name = "unknown"
        if parts[0] == 'apps':
            service_name = 'apps/osv'
        elif parts[0] == 'services' and len(parts) > 1:
            service_name = f"services/{parts[1]}"
            
        for file in files:
            if not file.endswith('.go'):
                continue
            
            filepath = os.path.join(root, file)
            try:
                with open(filepath, 'r', encoding='utf-8') as f:
                    content = f.read()
                    
                    for method, path in route_pattern.findall(content):
                        services_routes[service_name].append((method.upper(), path, filepath))
                        
            except Exception as e:
                print(f"Error reading {filepath}: {e}")

for d in dirs_to_scan:
    scan_directory(os.path.join(root_dir, d))

# Generate Markdown
with open(output_file, 'w', encoding='utf-8') as f:
    f.write("# Backend API Specifications\n\n")
    f.write("This document contains the automatically extracted REST API endpoints provided by the backend services and apps.\n\n")
    
    for service in sorted(services_routes.keys()):
        routes = services_routes[service]
        if not routes:
            continue
            
        # Deduplicate routes per service
        unique_routes = set()
        for r in routes:
            unique_routes.add((r[0], r[1], os.path.relpath(r[2], root_dir)))
        
        unique_routes = list(unique_routes)
        unique_routes.sort(key=lambda x: (x[1], x[0]))
        
        f.write(f"## {service}\n\n")
        f.write("| Method | Path | Source File |\n")
        f.write("|--------|------|-------------|\n")
        
        for method, path, rel_filepath in unique_routes:
            f.write(f"| `{method}` | `{path}` | `{rel_filepath}` |\n")
        
        f.write("\n")

print(f"API specs written to {output_file}")
