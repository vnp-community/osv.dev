import json
import re
from base_client import APIClient

c = APIClient()
c.login()

with open('../../ui/specs/api_endpoints.md', 'r') as f:
    content = f.read()

endpoints = []
for line in content.split('\n'):
    if '| `GET`' in line or '| GET ' in line:
        parts = line.split('|')
        if len(parts) > 3:
            path = parts[2].strip().replace('`', '').split(' ')[-1]
            if '{' in path: continue
            if 'export' in path or 'download' in path or 'stream' in path or 'auth/' in path: continue
            if path.startswith('/api/v1'):
                path = path[7:]
            endpoints.append(path)

bugs = []

def check_empty(data):
    if data is None: return True
    if isinstance(data, list) and len(data) == 0: return True
    if isinstance(data, dict):
        if len(data) == 0: return True
        if 'total' in data and data['total'] == 0: return True
        if 'count' in data and data['count'] == 0: return True
        if 'unread_count' in data and data['unread_count'] == 0: return True
        all_empty = True
        for k, v in data.items():
            if k in ['page', 'page_size']: continue
            if not check_empty(v):
                all_empty = False
                break
        return all_empty
    if data == 0: return True
    if data == "": return True
    return False

for ep in endpoints:
    try:
        if ep.startswith('/api/v2'):
            url = c.config.api_url.replace('/api/v1', '') + ep
            r = c.session.get(url)
        else:
            r = c.get(ep)
        
        if r.status_code >= 400:
            bugs.append(f"- [ ] [Lỗi HTTP {r.status_code}] `{ep}`: {r.text[:100].strip()}")
        else:
            try:
                data = r.json()
                if check_empty(data):
                    bugs.append(f"- [ ] [Dữ liệu trống/0 cần seed] `{ep}`: {json.dumps(data)[:150]}")
            except Exception as e:
                bugs.append(f"- [ ] [Lỗi JSON] `{ep}`: {r.text[:100].strip()}")
    except Exception as e:
        bugs.append(f"- [ ] [Lỗi Exception] `{ep}`: {str(e)}")

print("\n".join(bugs))
