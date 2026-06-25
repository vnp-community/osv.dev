#!/usr/bin/env python3
"""
test_all_endpoints.py — Parse api_endpoints.md and verify ALL endpoints & methods exist.

Strategy:
  - Reads every row from api_endpoints.md
  - Replaces path parameters with safe dummy values
  - Skips SSE stream endpoints (would block forever)
  - Calls the endpoint with the correct HTTP method
  - Treats status 404 on STATIC paths as FAIL (route not registered)
  - Treats status 404 on PARAMETRIC paths as PASS (route exists, resource not found)
  - Treats status 405 as FAIL (wrong method for a registered route)
  - Treats all other status codes (200, 400, 401, 403, 500, etc.) as PASS
    because it means the router handled the request
"""

import os
import re
import sys

from base_client import APIClient, TestResults

def main():
    client = APIClient()
    results = TestResults()
    
    # Login is optional but good to bypass some basic 401s if we want to see actual handler responses
    client.login()
    
    # Determine path to api_endpoints.md
    base_dir = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
    md_path = os.path.join(base_dir, 'ui', 'specs', 'api_endpoints.md')
    
    if not os.path.exists(md_path):
        print(f"Error: Could not find {md_path}")
        sys.exit(1)
        
    with open(md_path, 'r', encoding='utf-8') as f:
        lines = f.readlines()
        
    endpoints_to_test = []
    
    # Parse markdown table
    for line in lines:
        if line.strip().startswith('| `'):
            parts = line.split('|')
            if len(parts) >= 3:
                method = parts[1].replace('`', '').strip().upper()
                path = parts[2].replace('`', '').strip()
                endpoints_to_test.append((method, path))
                
    if not endpoints_to_test:
        print("No endpoints found in the markdown file.")
        sys.exit(1)
        
    print(f"Found {len(endpoints_to_test)} endpoints to test.\n")
    
    for method, original_path in endpoints_to_test:
        test_name = f"{method} {original_path}"
        
        # Replace ALL path parameters with dummy values
        path = original_path
        path = path.replace('{id}',         'test_id_123')
        path = path.replace('{cveId}',      'CVE-2023-0001')
        path = path.replace('{vendor}',     'test_vendor')
        path = path.replace('{product}',    'test_product')
        path = path.replace('{engId}',      'test_eng_123')
        path = path.replace('{findingId}',  'test_finding_123')
        path = path.replace('{deliveryId}', 'test_delivery_123')

        # Skip SSE streaming endpoints (would block forever)
        if path.endswith('/stream'):
            results.record_skip(test_name, "SSE stream endpoint — skipped to avoid blocking")
            continue
        
        # Determine v1 vs v2
        is_v2 = False
        if path.startswith('/api/v2'):
            is_v2 = True
            api_path = path[len('/api/v2'):]
        elif path.startswith('/api/v1'):
            api_path = path[len('/api/v1'):]
        else:
            api_path = path
            
        if not api_path.startswith('/'):
            api_path = '/' + api_path
            
        try:
            if method == 'GET':
                resp = client.get(api_path, v2=is_v2)
            elif method == 'POST':
                resp = client.post(api_path, v2=is_v2, body={})
            elif method == 'PUT':
                resp = client.put(api_path, v2=is_v2, body={})
            elif method == 'PATCH':
                resp = client.patch(api_path, v2=is_v2, body={})
            elif method == 'DELETE':
                resp = client.delete(api_path, v2=is_v2)
            else:
                results.record_skip(test_name, f"Unsupported method {method}")
                continue
                
            # If the router returns 404, it might mean the endpoint doesn't exist
            # If it returns 405, the endpoint exists but method is wrong
            # Note: A resource like /findings/test_id_123 returning 404 is normal if it checks DB first.
            # But usually, frameworks might return 400 for bad UUID, or we just accept 404 as long as we know it's a "Not Found in DB" 404 and not a "Route Not Found" 404.
            # However, standardizing this is hard. We'll flag 405 specifically, and warn on 404.
            
            if resp.status_code == 405:
                results.record_fail(test_name, f"Method Not Allowed (405). Route might exist but not for {method}")
            elif resp.status_code == 404 and '{' not in original_path:
                # Static route returning 404 usually means the route is not registered.
                # Exception: endpoints that may require query params to function, log it as warning.
                results.record_fail(test_name, "Static route returned 404 Not Found (route may be missing)")
            else:
                # Any other status (200, 400, 401, 403, 409, 500, etc.) means the router
                # handled the request — the endpoint exists.
                results.record_pass(test_name)
                
        except Exception as e:
            results.record_fail(test_name, f"Request exception: {str(e)}")
            
    results.summary()
    sys.exit(results.exit_code())

if __name__ == '__main__':
    main()
