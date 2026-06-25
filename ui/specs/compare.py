import re

with open('/Users/binhnt/Lab/sec/cve/osv.dev/ui/specs/api_endpoints.md', 'r') as f:
    md_content = f.readlines()

with open('/Users/binhnt/Lab/sec/cve/osv.dev/ui/specs/openapi.yaml', 'r') as f:
    yaml_content = f.readlines()

md_endpoints = []
for line in md_content:
    if line.startswith('| `'):
        parts = line.split('|')
        if len(parts) >= 3:
            method = parts[1].replace('`', '').strip().upper()
            path = parts[2].replace('`', '').strip()
            path = re.sub(r'^/api/v[12]', '', path)
            if path == '': path = '/'
            md_endpoints.append(f"{method} {path}")

yaml_endpoints = []
current_path = ""
for line in yaml_content:
    if line.startswith('  /') and not line.startswith('    '):
        current_path = line.replace(':', '').strip()
    elif line.startswith('    get:') or line.startswith('    post:') or line.startswith('    put:') or line.startswith('    delete:') or line.startswith('    patch:'):
        method = line.replace(':', '').strip().upper()
        yaml_endpoints.append(f"{method} {current_path}")

missing = set(md_endpoints) - set(yaml_endpoints)
print("Endpoints in MD but missing in YAML:")
for e in sorted(list(missing)):
    print(e)
