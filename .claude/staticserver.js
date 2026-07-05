// Zero-dependency static file server for previewing generated artifacts
// (e.g. tmp/<run>/dashboard.html) in the Claude Code browser preview.
// Avoids process.cwd() (the preview sandbox denies getcwd()) by resolving
// the served root from __dirname instead.
const http = require('http');
const fs = require('fs');
const path = require('path');

const root = path.join(__dirname, '..'); // repo root
const port = 8743;

const types = { '.html': 'text/html', '.json': 'application/json', '.txt': 'text/plain' };

http.createServer((req, res) => {
  const reqPath = decodeURIComponent(req.url.split('?')[0]);
  const filePath = path.join(root, reqPath);
  if (!filePath.startsWith(root)) {
    res.writeHead(403);
    res.end('forbidden');
    return;
  }
  fs.readFile(filePath, (err, data) => {
    if (err) {
      res.writeHead(404);
      res.end('not found');
      return;
    }
    res.writeHead(200, { 'Content-Type': types[path.extname(filePath)] || 'application/octet-stream' });
    res.end(data);
  });
}).listen(port, () => console.log('listening on ' + port));
