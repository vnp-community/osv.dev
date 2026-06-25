import json
import re
from base_client import APIClient

c = APIClient()
c.login()

endpoints = [
    '/dashboard?period=30d',
    '/dashboard/risk-trend?period=30d',
    '/notifications',
    '/findings',
    '/findings/stats',
    '/scans',
    '/scans/stats',
    '/assets',
    '/products',
    '/products/grades',
    '/cve',
    '/kev',
    '/epss/cve-2023-1234',
    '/taxonomy/cwe',
    '/taxonomy/capec',
    '/engagements',
    '/admin/users',
    '/audit-log',
    '/search/recent',
    '/search/suggested',
    '/profile',
    '/profile/sessions',
    '/profile/notifications/settings',
    '/webhooks',
    '/integrations/jira',
    '/ai/triage/queue',
    '/ai/reports'
]

bugs = []

def check_empty(data):
    if data is None: return True
    if isinstance(data, list) and len(data) == 0: return True
    if isinstance(data, dict):
        if len(data) == 0: return True
        # Check specific pagination formats
        if 'total' in data and data['total'] == 0: return True
        if 'count' in data and data['count'] == 0: return True
        # If it's a dict, we might want to check if all lists/dicts inside are empty
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
        r = c.get(ep)
        if r.status_code >= 400:
            bugs.append(f"[Lỗi HTTP {r.status_code}] {ep}: {r.text[:100]}")
        else:
            try:
                data = r.json()
                if check_empty(data):
                    bugs.append(f"[Dữ liệu trống/0 cần seed] {ep}: {json.dumps(data)[:150]}")
            except Exception as e:
                bugs.append(f"[Lỗi JSON] {ep}: {r.text[:100]}")
    except Exception as e:
        bugs.append(f"[Lỗi Exception] {ep}: {str(e)}")

print("\n".join(bugs))
