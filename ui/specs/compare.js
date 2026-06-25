const fs = require('fs');

const mdContent = fs.readFileSync('/Users/binhnt/Lab/sec/cve/osv.dev/ui/specs/api_endpoints.md', 'utf8');
const yamlContent = fs.readFileSync('/Users/binhnt/Lab/sec/cve/osv.dev/ui/specs/openapi.yaml', 'utf8');

// Parse MD for endpoints
const mdEndpoints = [];
const mdLines = mdContent.split('\n');
for (const line of mdLines) {
  if (line.startsWith('| `')) {
    const parts = line.split('|');
    if (parts.length >= 3) {
      const methodStr = parts[1].trim().replace(/`/g, '');
      let pathStr = parts[2].trim().replace(/`/g, '');
      
      // Clean up path, remove /api/v1 or /api/v2
      pathStr = pathStr.replace(/^\/api\/v[12]/, '');
      if (pathStr === '') pathStr = '/';
      
      mdEndpoints.push(`${methodStr.toUpperCase()} ${pathStr}`);
    }
  }
}

// Parse YAML for endpoints
const yamlEndpoints = [];
const yamlLines = yamlContent.split('\n');
let currentPath = '';
for (const line of yamlLines) {
  if (line.startsWith('  /')) {
    currentPath = line.trim().replace(':', '');
  } else if (line.startsWith('    get:') || line.startsWith('    post:') || line.startsWith('    put:') || line.startsWith('    delete:') || line.startsWith('    patch:')) {
    const method = line.trim().replace(':', '').toUpperCase();
    yamlEndpoints.push(`${method} ${currentPath}`);
  }
}

const missingInYaml = mdEndpoints.filter(e => !yamlEndpoints.includes(e));
console.log("Endpoints in MD but missing in YAML:");
missingInYaml.forEach(e => console.log(e));

