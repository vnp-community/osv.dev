from base_client import APIClient
client = APIClient()
client.login()
resp = client.get("/dashboard/sla")
print(resp.json())
