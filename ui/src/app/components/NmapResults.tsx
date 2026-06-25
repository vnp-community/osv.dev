import { useState } from "react";
import { Server, Shield, ChevronRight, Wifi } from "lucide-react";

const hosts = [
  { ip: "10.0.1.45", hostname: "webserver01.prod", os: "Ubuntu 22.04", riskScore: 9.8, ports: [{ port: 22, proto: "tcp", service: "SSH", version: "OpenSSH 8.9p1", state: "open" }, { port: 80, proto: "tcp", service: "HTTP", version: "nginx 1.22.1", state: "open" }, { port: 443, proto: "tcp", service: "HTTPS", version: "nginx 1.22.1", state: "open" }, { port: 8080, proto: "tcp", service: "HTTP-ALT", version: "Apache Tomcat 9.0.58", state: "open" }], cves: ["CVE-2025-44228", "CVE-2025-18935", "CVE-2025-33127"] },
  { ip: "10.0.2.11", hostname: "db01.prod", os: "CentOS 8", riskScore: 8.9, ports: [{ port: 22, proto: "tcp", service: "SSH", version: "OpenSSH 7.4p1", state: "open" }, { port: 3306, proto: "tcp", service: "MySQL", version: "MySQL 8.0.32", state: "open" }], cves: ["CVE-2025-44228", "CVE-2025-09876"] },
  { ip: "10.0.3.22", hostname: "api-gw.prod", os: "Debian 11", riskScore: 8.1, ports: [{ port: 443, proto: "tcp", service: "HTTPS", version: "nginx 1.22.1", state: "open" }, { port: 8443, proto: "tcp", service: "HTTPS-ALT", version: "Kong 3.4.0", state: "open" }], cves: ["CVE-2025-18935"] },
  { ip: "10.0.4.10", hostname: "cache01.prod", os: "Alpine Linux", riskScore: 3.2, ports: [{ port: 6379, proto: "tcp", service: "Redis", version: "Redis 7.0.12", state: "open" }], cves: [] },
  { ip: "10.0.5.55", hostname: "k8s-master-01", os: "Ubuntu 20.04", riskScore: 6.8, ports: [{ port: 6443, proto: "tcp", service: "HTTPS", version: "k8s API 1.28", state: "open" }, { port: 2379, proto: "tcp", service: "etcd", version: "etcd 3.5.9", state: "open" }, { port: 10250, proto: "tcp", service: "Kubelet", version: "Kubelet 1.28", state: "open" }], cves: [] },
];

const RISK_COLOR = (s: number) => s >= 9 ? "#EF4444" : s >= 7 ? "#F97316" : s >= 4 ? "#EAB308" : "#10B981";

export function NmapResults({ onBack }: { onBack?: () => void }) {
  const [selected, setSelected] = useState(hosts[0]);

  return (
    <div className="flex flex-1 overflow-hidden" style={{ background: "#0B1020" }}>
      {/* Host list */}
      <div className="w-64 flex-shrink-0 overflow-y-auto" style={{ background: "#0F1629", borderRight: "1px solid rgba(255,255,255,0.06)" }}>
        <div className="px-4 py-3" style={{ borderBottom: "1px solid rgba(255,255,255,0.06)" }}>
          <div style={{ color: "#6B7280", fontSize: 11, fontWeight: 600, letterSpacing: 1 }}>HOSTS ({hosts.length})</div>
        </div>
        {hosts.map(h => (
          <button key={h.ip} onClick={() => setSelected(h)}
            className="w-full flex items-start gap-3 px-4 py-3 text-left transition-all"
            style={{ background: selected.ip === h.ip ? "rgba(79,140,255,0.1)" : "transparent", borderBottom: "1px solid rgba(255,255,255,0.04)", borderLeft: selected.ip === h.ip ? "2px solid #4F8CFF" : "2px solid transparent" }}
          >
            <div className="w-2 h-2 rounded-full mt-1.5 flex-shrink-0" style={{ background: RISK_COLOR(h.riskScore) }} />
            <div className="min-w-0">
              <div style={{ color: "#E5E7EB", fontSize: 12, fontFamily: "monospace" }}>{h.ip}</div>
              <div style={{ color: "#6B7280", fontSize: 11 }} className="truncate">{h.hostname}</div>
              <div className="flex items-center gap-2 mt-1">
                <span style={{ color: "#6B7280", fontSize: 10 }}>{h.ports.length} ports</span>
                {h.cves.length > 0 && <span style={{ color: "#EF4444", fontSize: 10 }}>{h.cves.length} CVEs</span>}
              </div>
            </div>
          </button>
        ))}
      </div>

      {/* Host detail */}
      {selected && (
        <div className="flex-1 overflow-y-auto p-5">
          <div className="flex items-center gap-4 mb-5">
            <div className="w-12 h-12 rounded-xl flex items-center justify-center" style={{ background: RISK_COLOR(selected.riskScore) + "20" }}>
              <Server size={22} color={RISK_COLOR(selected.riskScore)} />
            </div>
            <div>
              <div style={{ color: "#E5E7EB", fontSize: 18, fontFamily: "monospace", fontWeight: 700 }}>{selected.ip}</div>
              <div style={{ color: "#6B7280", fontSize: 13 }}>{selected.hostname} · {selected.os}</div>
            </div>
            <div className="ml-auto flex items-center gap-3">
              <div className="text-center">
                <div style={{ color: RISK_COLOR(selected.riskScore), fontSize: 24, fontWeight: 800 }}>{selected.riskScore}</div>
                <div style={{ color: "#6B7280", fontSize: 10 }}>Risk Score</div>
              </div>
            </div>
          </div>

          {/* Open ports */}
          <div className="rounded-2xl mb-4" style={{ background: "#151B2F", border: "1px solid rgba(255,255,255,0.07)" }}>
            <div className="px-5 py-3" style={{ borderBottom: "1px solid rgba(255,255,255,0.06)" }}>
              <div style={{ color: "#E5E7EB", fontSize: 13, fontWeight: 600 }}>Open Ports ({selected.ports.length})</div>
            </div>
            <table className="w-full">
              <thead>
                <tr style={{ borderBottom: "1px solid rgba(255,255,255,0.05)" }}>
                  {["Port", "Protocol", "Service", "Version", "State"].map(h => (
                    <th key={h} className="px-4 py-2 text-left" style={{ color: "#6B7280", fontSize: 11, fontWeight: 600 }}>{h}</th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {selected.ports.map(p => (
                  <tr key={p.port} style={{ borderBottom: "1px solid rgba(255,255,255,0.04)" }}>
                    <td className="px-4 py-2.5"><span style={{ color: "#4F8CFF", fontSize: 13, fontFamily: "monospace", fontWeight: 600 }}>{p.port}</span></td>
                    <td className="px-4 py-2.5"><span style={{ color: "#6B7280", fontSize: 12 }}>{p.proto}</span></td>
                    <td className="px-4 py-2.5"><span style={{ color: "#E5E7EB", fontSize: 12 }}>{p.service}</span></td>
                    <td className="px-4 py-2.5"><span style={{ color: "#9CA3AF", fontSize: 12 }}>{p.version}</span></td>
                    <td className="px-4 py-2.5"><span style={{ color: "#10B981", fontSize: 11 }}>open</span></td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          {/* Detected CVEs */}
          {selected.cves.length > 0 && (
            <div className="rounded-2xl" style={{ background: "#151B2F", border: "1px solid rgba(239,68,68,0.2)" }}>
              <div className="px-5 py-3" style={{ borderBottom: "1px solid rgba(255,255,255,0.06)" }}>
                <div style={{ color: "#EF4444", fontSize: 13, fontWeight: 600 }}>Detected CVEs ({selected.cves.length})</div>
              </div>
              <div className="p-4 flex flex-col gap-2">
                {selected.cves.map(cve => (
                  <div key={cve} className="flex items-center gap-3 px-3 py-2.5 rounded-xl" style={{ background: "rgba(239,68,68,0.07)", border: "1px solid rgba(239,68,68,0.12)" }}>
                    <Shield size={13} color="#EF4444" />
                    <span style={{ color: "#EF4444", fontSize: 12, fontFamily: "monospace", fontWeight: 600 }}>{cve}</span>
                    <span className="ml-auto px-2 py-0.5 rounded" style={{ background: "rgba(239,68,68,0.2)", color: "#EF4444", fontSize: 10 }}>CRITICAL</span>
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
