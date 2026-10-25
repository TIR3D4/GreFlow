"use strict";
const http = require("node:http");
const fs = require("node:fs");
const path = require("node:path");
const root = __dirname;
const port = Number(process.env.GREFLOW_PREVIEW_PORT || 8787);
if (!Number.isInteger(port) || port < 1 || port > 65535)
  throw new Error("Invalid preview port");
const types = {
  ".html": "text/html; charset=utf-8",
  ".css": "text/css; charset=utf-8",
  ".js": "text/javascript; charset=utf-8",
  ".woff2": "font/woff2",
};
http
  .createServer((req, res) => {
    if (!["GET", "HEAD"].includes(req.method)) {
      res.writeHead(405);
      res.end();
      return;
    }
    let requestPath;
    try {
      requestPath = decodeURIComponent(
        new URL(req.url, "http://localhost").pathname,
      );
    } catch {
      res.writeHead(400);
      res.end();
      return;
    }
    const file = path.resolve(
      root,
      "." + (requestPath === "/" ? "/index.html" : requestPath),
    );
    const relative = path.relative(root, file);
    if (
      relative.startsWith("..") ||
      path.isAbsolute(relative) ||
      ![".html", ".css", ".js", ".woff2"].includes(path.extname(file))
    ) {
      res.writeHead(404);
      res.end();
      return;
    }
    fs.readFile(file, (err, data) => {
      if (err) {
        res.writeHead(404);
        res.end();
        return;
      }
      res.writeHead(200, {
        "Content-Type": types[path.extname(file)],
        "Cache-Control": "no-store",
        "X-Content-Type-Options": "nosniff",
        "Referrer-Policy": "no-referrer",
        "Content-Security-Policy":
          "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; font-src 'self'; img-src 'self' data:; connect-src 'none'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'",
      });
      res.end(req.method === "HEAD" ? undefined : data);
    });
  })
  .listen(port, "127.0.0.1", () =>
    console.log(
      `GreFlow DEMO preview: http://127.0.0.1:${port} (no server management)`,
    ),
  );
