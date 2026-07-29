const fs = require("fs");
const path = require("path");

const baseURL = process.env.ORDER_BASE_URL || "http://127.0.0.1:8090";
const outDir = process.env.ORDER_STOCK_RECONCILE_OUT || path.join(process.cwd(), "reports", "baseline-run");
const adminUsername = process.env.ORDER_ADMIN_USERNAME || "admin";
const adminPassword = process.env.ORDER_ADMIN_PASSWORD || "admin123456";

fs.mkdirSync(outDir, { recursive: true });

async function request(name, url, options = {}) {
  const res = await fetch(url, options);
  const text = await res.text();
  let body;
  try {
    body = JSON.parse(text);
  } catch {
    body = text;
  }
  const output = { status: res.status, body, traceID: res.headers.get("x-trace-id") || "" };
  fs.writeFileSync(path.join(outDir, name), JSON.stringify(output, null, 2));
  return output;
}

function postJSON(name, url, body, token = "") {
  const headers = { "Content-Type": "application/json" };
  if (token) headers.Authorization = `Bearer ${token}`;
  return request(name, url, { method: "POST", headers, body: JSON.stringify(body) });
}

async function main() {
  const stamp = Date.now();
  const adminLogin = await postJSON("stock-reconcile-admin-login.json", `${baseURL}/api/auth/login`, {
    username: adminUsername,
    password: adminPassword,
  });
  const adminToken = adminLogin.body.data.token;
  const register = await postJSON("stock-reconcile-register.json", `${baseURL}/api/auth/register`, {
    username: `stock${stamp}`,
    password: "123456",
  });
  const token = register.body.data.token;

  const product = await postJSON("stock-reconcile-product.json", `${baseURL}/api/products`, {
    name: `stock-phone-${stamp}`,
    price: 199900,
    stock: 100,
  }, adminToken);
  const activity = await postJSON("stock-reconcile-activity.json", `${baseURL}/api/activities`, {
    product_id: product.body.data.id,
    name: `stock-sale-${stamp}`,
    price: 189900,
    stock: 8,
    duration_seconds: 3600,
  }, adminToken);

  const reconcile = await postJSON("stock-reconcile-check.json", `${baseURL}/api/ops/stock/reconcile`, {
    activity_id: activity.body.data.id,
    repair: false,
  }, adminToken);

  const output = {
    adminLogin: adminLogin.status,
    register: register.status,
    product: product.status,
    activity: activity.status,
    reconcile: reconcile.status,
    activityID: activity.body.data.id,
    checked: reconcile.body.data.checked,
    missing: reconcile.body.data.missing,
    mismatched: reconcile.body.data.mismatched,
    repaired: reconcile.body.data.repaired,
    traceID: reconcile.traceID,
  };
  fs.writeFileSync(path.join(outDir, "stock-reconcile-result.json"), JSON.stringify(output, null, 2));
  console.log(JSON.stringify(output, null, 2));

  if (reconcile.status !== 200) {
    throw new Error(`reconcile status = ${reconcile.status}`);
  }
  if (output.checked !== 1 || output.missing !== 0 || output.mismatched !== 0) {
    throw new Error(`expected prewarmed redis stock to match mysql, got ${JSON.stringify(output)}`);
  }
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
