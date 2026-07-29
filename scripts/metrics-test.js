const fs = require("fs");
const path = require("path");

const baseURL = process.env.ORDER_BASE_URL || "http://127.0.0.1:8090";
const outDir = process.env.ORDER_METRICS_OUT || path.join(process.cwd(), "reports", "baseline-run");
const adminUsername = process.env.ORDER_ADMIN_USERNAME || "admin";
const adminPassword = process.env.ORDER_ADMIN_PASSWORD || "admin123456";

fs.mkdirSync(outDir, { recursive: true });

async function request(url, options = {}) {
  const res = await fetch(url, options);
  const text = await res.text();
  let body = text;
  try {
    body = JSON.parse(text);
  } catch {
    // /metrics is plain text.
  }
  return { status: res.status, body, traceID: res.headers.get("x-trace-id") || "" };
}

function postJSON(url, body, token = "") {
  const headers = { "Content-Type": "application/json" };
  if (token) headers.Authorization = `Bearer ${token}`;
  return request(url, { method: "POST", headers, body: JSON.stringify(body) });
}

function assertStatus(name, response, expected) {
  if (response.status !== expected) {
    throw new Error(`${name} expected ${expected}, got ${response.status}: ${JSON.stringify(response.body)}`);
  }
}

async function main() {
  const stamp = Date.now();

  const health = await request(`${baseURL}/health`);
  assertStatus("health", health, 200);

  const adminLogin = await postJSON(`${baseURL}/api/auth/login`, {
    username: adminUsername,
    password: adminPassword,
  });
  assertStatus("admin login", adminLogin, 200);
  const adminToken = adminLogin.body.data.token;

  const register = await postJSON(`${baseURL}/api/auth/register`, {
    username: `metrics${stamp}`,
    password: "123456",
  });
  assertStatus("register", register, 201);
  const token = register.body.data.token;

  const product = await postJSON(`${baseURL}/api/products`, {
    name: `metrics-phone-${stamp}`,
    price: 199900,
    stock: 20,
  }, adminToken);
  assertStatus("product", product, 201);

  const activity = await postJSON(`${baseURL}/api/activities`, {
    product_id: product.body.data.id,
    name: `metrics-sale-${stamp}`,
    price: 189900,
    stock: 5,
    duration_seconds: 3600,
  }, adminToken);
  assertStatus("activity", activity, 201);

  const order = await postJSON(`${baseURL}/api/orders`, {
    activity_id: activity.body.data.id,
    request_id: `metrics-${stamp}`,
  }, token);
  assertStatus("order", order, 202);

  const metrics = await request(`${baseURL}/metrics`);
  assertStatus("metrics", metrics, 200);
  const text = metrics.body;

  const checks = [
    "go_order_http_requests_total",
    'path="/health"',
    'path="/api/orders"',
    'go_order_business_events_total{event="activity_order",result="accepted"}',
  ];
  for (const item of checks) {
    if (!text.includes(item)) {
      throw new Error(`metrics output missing ${item}`);
    }
  }

  const output = {
    health: health.status,
    adminLogin: adminLogin.status,
    register: register.status,
    product: product.status,
    activity: activity.status,
    order: order.status,
    metrics: metrics.status,
    orderTraceID: order.traceID,
  };
  fs.writeFileSync(path.join(outDir, "metrics-result.json"), JSON.stringify(output, null, 2));
  fs.writeFileSync(path.join(outDir, "metrics.txt"), text);
  console.log(JSON.stringify(output, null, 2));
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
