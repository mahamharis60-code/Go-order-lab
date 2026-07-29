const fs = require("fs");
const path = require("path");

const baseURL = process.env.ORDER_BASE_URL || "http://127.0.0.1:8090";
const outDir = process.env.ORDER_RATE_LIMIT_OUT || path.join(process.cwd(), "reports", "baseline-run");
const concurrency = Number(process.env.ORDER_RATE_LIMIT_CONCURRENCY || 12);
const adminUsername = process.env.ORDER_ADMIN_USERNAME || "admin";
const adminPassword = process.env.ORDER_ADMIN_PASSWORD || "admin123456";

fs.mkdirSync(outDir, { recursive: true });

async function request(url, options = {}) {
  const res = await fetch(url, options);
  const text = await res.text();
  let body;
  try {
    body = JSON.parse(text);
  } catch {
    body = text;
  }
  return { status: res.status, body, traceID: res.headers.get("x-trace-id") || "" };
}

function postJSON(url, body, token = "") {
  const headers = { "Content-Type": "application/json" };
  if (token) headers.Authorization = `Bearer ${token}`;
  return request(url, { method: "POST", headers, body: JSON.stringify(body) });
}

async function main() {
  const stamp = Date.now();
  const adminLogin = await postJSON(`${baseURL}/api/auth/login`, {
    username: adminUsername,
    password: adminPassword,
  });
  const adminToken = adminLogin.body.data.token;
  const register = await postJSON(`${baseURL}/api/auth/register`, {
    username: `limit${stamp}`,
    password: "123456",
  });
  const token = register.body.data.token;

  const product = await postJSON(`${baseURL}/api/products`, {
    name: `limit-phone-${stamp}`,
    price: 199900,
    stock: 100,
  }, adminToken);
  const activity = await postJSON(`${baseURL}/api/activities`, {
    product_id: product.body.data.id,
    name: `limit-sale-${stamp}`,
    price: 189900,
    stock: 100,
    duration_seconds: 3600,
  }, adminToken);

  const calls = Array.from({ length: concurrency }, (_, i) => postJSON(`${baseURL}/api/orders`, {
    activity_id: activity.body.data.id,
    request_id: `limit-${stamp}-${i}`,
  }, token));
  const results = await Promise.all(calls);
  const summary = {};
  for (const item of results) {
    summary[item.status] = (summary[item.status] || 0) + 1;
  }

  const output = { adminLogin: adminLogin.status, summary, traceIDs: results.map((item) => item.traceID).filter(Boolean) };
  fs.writeFileSync(path.join(outDir, "order-rate-limit-result.json"), JSON.stringify({
    output,
    results,
  }, null, 2));
  console.log(JSON.stringify(output, null, 2));

  if (!summary["429"]) {
    throw new Error(`expected at least one 429, got ${JSON.stringify(summary)}`);
  }
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
