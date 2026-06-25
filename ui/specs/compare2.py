import re

with open('/Users/binhnt/Lab/sec/cve/osv.dev/ui/specs/api_endpoints.md', 'r') as f:
    md_content = f.readlines()

with open('/Users/binhnt/Lab/sec/cve/osv.dev/ui/specs/openapi.yaml', 'r') as f:
    yaml_content = f.readlines()

md_paths = set()
for line in md_content:
    if line.startswith('| `'):
        parts = line.split('|')
        if len(parts) >= 3:
            path = parts[2].replace('`', '').strip()
            path = re.sub(r'^/api/v[12]', '', path)
            if path == '': path = '/'
            md_paths.add(path)

yaml_paths = set()
current_path = ""
for line in yaml_content:
    if line.startswith('  /') and not line.startswith('    '):
        current_path = line.replace(':', '').strip()
        yaml_paths.add(current_path)

missing_in_yaml = md_paths - yaml_paths
print("Paths in MD but missing in YAML:")
for p in sorted(list(missing_in_yaml)):
    print(p)

