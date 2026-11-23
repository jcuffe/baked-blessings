// Serves the built storefront (the same worker bundle Oxygen runs) from plain Node,
// so it can ship as a container: Lambda (through the Lambda Web Adapter), Fargate or
// a Kubernetes pod. No dependencies beyond Node 22.
//
//   node container/server.mjs        (after `pnpm build`)
//
// Env: PORT (default 8080), SESSION_SECRET, PUBLIC_STORE_DOMAIN,
// PUBLIC_STOREFRONT_API_TOKEN and the other variables Oxygen would inject.
// ORIGIN_VERIFY_SECRET, when set, makes the server refuse requests that don't carry
// it in x-origin-verify, so only the CDN in front of it can reach the app.
import {timingSafeEqual} from 'node:crypto';
import {createServer} from 'node:http';
import {createReadStream} from 'node:fs';
import {stat} from 'node:fs/promises';
import {extname, join, normalize, resolve} from 'node:path';
import {Readable} from 'node:stream';
import {fileURLToPath} from 'node:url';

const root = resolve(fileURLToPath(new URL('..', import.meta.url)));
const clientDir = join(root, 'dist', 'client');
const {default: worker} = await import(join(root, 'dist', 'server', 'index.js'));

const port = Number(process.env.PORT ?? 8080);
const originSecret = process.env.ORIGIN_VERIFY_SECRET
  ? Buffer.from(process.env.ORIGIN_VERIFY_SECRET)
  : null;

function fromOrigin(req) {
  if (!originSecret) return true;
  const given = Buffer.from(String(req.headers['x-origin-verify'] ?? ''));
  return given.length === originSecret.length && timingSafeEqual(given, originSecret);
}

const CONTENT_TYPES = {
  '.css': 'text/css; charset=utf-8',
  '.js': 'text/javascript; charset=utf-8',
  '.mjs': 'text/javascript; charset=utf-8',
  '.json': 'application/json',
  '.map': 'application/json',
  '.svg': 'image/svg+xml',
  '.png': 'image/png',
  '.jpg': 'image/jpeg',
  '.jpeg': 'image/jpeg',
  '.webp': 'image/webp',
  '.avif': 'image/avif',
  '.ico': 'image/x-icon',
  '.woff': 'font/woff',
  '.woff2': 'font/woff2',
  '.txt': 'text/plain; charset=utf-8',
};

// Files under dist/client, such as /assets/app-1234.css. Hashed assets are cached for a year.
async function serveStatic(req, res, pathname) {
  if (req.method !== 'GET' && req.method !== 'HEAD') return false;
  const file = normalize(join(clientDir, decodeURIComponent(pathname)));
  if (!file.startsWith(clientDir + '/')) return false;
  const info = await stat(file).catch(() => null);
  if (!info?.isFile()) return false;
  res.writeHead(200, {
    'content-type': CONTENT_TYPES[extname(file)] ?? 'application/octet-stream',
    'content-length': info.size,
    'cache-control': pathname.startsWith('/assets/')
      ? 'public, max-age=31536000, immutable'
      : 'public, max-age=3600',
  });
  if (req.method === 'HEAD') res.end();
  else createReadStream(file).pipe(res);
  return true;
}

function toRequest(req) {
  const proto = req.headers['x-forwarded-proto']?.split(',')[0] ?? 'http';
  const host = req.headers['x-forwarded-host'] ?? req.headers.host ?? 'localhost';
  const headers = new Headers();
  for (const [name, value] of Object.entries(req.headers)) {
    if (value === undefined || name === 'x-origin-verify') continue;
    for (const v of Array.isArray(value) ? value : [value]) headers.append(name, v);
  }
  // Oxygen passes the buyer's IP to Hydrogen in this header.
  if (!headers.has('oxygen-buyer-ip')) {
    const ip = req.headers['x-forwarded-for']?.split(',')[0].trim() ?? req.socket.remoteAddress;
    if (ip) headers.set('oxygen-buyer-ip', ip);
  }
  const hasBody = req.method !== 'GET' && req.method !== 'HEAD';
  return new Request(`${proto}://${host}${req.url}`, {
    method: req.method,
    headers,
    body: hasBody ? Readable.toWeb(req) : undefined,
    duplex: hasBody ? 'half' : undefined,
  });
}

async function send(res, response, method) {
  const headers = {};
  response.headers.forEach((value, name) => {
    if (name !== 'set-cookie') headers[name] = value;
  });
  const cookies = response.headers.getSetCookie();
  if (cookies.length) headers['set-cookie'] = cookies;
  res.writeHead(response.status, response.statusText, headers);
  if (!response.body || method === 'HEAD') return res.end();
  Readable.fromWeb(response.body).pipe(res);
}

const executionContext = {
  waitUntil: (promise) => promise.catch((error) => console.error(error)),
  passThroughOnException() {},
};

const server = createServer(async (req, res) => {
  try {
    const {pathname} = new URL(req.url, 'http://localhost');
    // Liveness for Lambda Web Adapter, load balancers and Kubernetes probes.
    if (pathname === '/healthz') {
      res.writeHead(200, {'content-type': 'text/plain'});
      return res.end('ok');
    }
    if (!fromOrigin(req)) {
      res.writeHead(403, {'content-type': 'text/plain'});
      return res.end('Forbidden');
    }
    if (await serveStatic(req, res, pathname)) return;
    const response = await worker.fetch(toRequest(req), process.env, executionContext);
    await send(res, response, req.method);
  } catch (error) {
    console.error(error);
    if (!res.headersSent) res.writeHead(500, {'content-type': 'text/plain'});
    res.end('An unexpected error occurred');
  }
});

server.listen(port, () => console.log(`Storefront listening on http://localhost:${port}`));

for (const signal of ['SIGTERM', 'SIGINT']) {
  process.on(signal, () => server.close(() => process.exit(0)));
}
