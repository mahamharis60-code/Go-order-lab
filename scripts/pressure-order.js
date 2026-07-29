const fs = require("fs");
const path = require("path");

const baseURL = process.env.ORDER_BASE_URL || "http://127.0.0.1:8090";
const outDir = process.env.ORDER_PRESSURE_OUT || path.join(process.cwd(), "reports", "baseline-run");
const adminUsername = process.env.ORDER_ADMIN_USERNAME || "admin";
const adminPassword = process.env.ORDER_ADMIN_PASSWORD || "admin123456";
const pressureConfig = parsePressureConfig(process.env);

fs.mkdirSync(outDir, { recursive: true });

function parsePressureConfig(env) {
  return {
    sameUserConcurrency: positiveInt(env.ORDER_PRESSURE_SAME_USER_CONCURRENCY, 20),
    sameUserStock: positiveInt(env.ORDER_PRESSURE_SAME_USER_STOCK, 5),
    multiUsers: positiveInt(env.ORDER_PRESSURE_MULTI_USERS, 10),
    multiStock: positiveInt(env.ORDER_PRESSURE_MULTI_STOCK, 3),
    registerConcurrency: positiveInt(env.ORDER_PRESSURE_REGISTER_CONCURRENCY, 20),
  };
}

function positiveInt(value, fallback) {
  const parsed = Number(value);
  if (!Number.isInteger(parsed) || parsed <= 0) {
    return fallback;
  }
  return parsed;
}

async function request(url, options = {}) {
  const res = await fetch(url, options);
  const text = await res.text();
  let body;
  try {
    body = JSON.parse(text);
  } catch {
    body = text;
  }
  return { status: res.status, body };
}

function postJSON(url, body, token = "") {
  const headers = { "Content-Type": "application/json" };
  if (token) headers.Authorization = `Bearer ${token}`;
  return request(url, { method: "POST", headers, body: JSON.stringify(body) });
}

async function register(prefix) {
  const res = await postJSON(`${baseURL}/api/auth/register`, {
    username: `${prefix}${Date.now()}${Math.floor(Math.random() * 100000)}`,
    password: "123456",
  });
  assertStatus("register", res, 201);
  return res.body.data.token;
}

async function adminLogin() {
  const res = await postJSON(`${baseURL}/api/auth/login`, {
    username: adminUsername,
    password: adminPassword,
  });
  assertStatus("admin login", res, 200);
  return res.body.data.token;
}

async function createActivity(adminToken, stock, suffix) {
  const product = await postJSON(`${baseURL}/api/products`, {
    name: `pressure-phone-${suffix}`,
    price: 199900,
    stock: 100,
  }, adminToken);
  assertStatus("create product", product, 201);
  const activity = await postJSON(`${baseURL}/api/activities`, {
    product_id: product.body.data.id,
    name: `pressure-sale-${suffix}`,
    price: 189900,
    stock,
    duration_seconds: 3600,
  }, adminToken);
  assertStatus("create activity", activity, 201);
  return activity.body.data.id;
}

function assertStatus(name, response, expected) {
  if (response.status !== expected) {
    throw new Error(`${name} expected ${expected}, got ${response.status}: ${JSON.stringify(response.body)}`);
  }
  if (!response.body || !response.body.data) {
    throw new Error(`${name} response missing data: ${JSON.stringify(response)}`);
  }
}

async function mapWithConcurrency(items, concurrency, mapper) {
  const results = new Array(items.length);
  let nextIndex = 0;
  const workers = Array.from({ length: Math.min(concurrency, items.length) }, async () => {
    for (;;) {
      const current = nextIndex;
      nextIndex += 1;
      if (current >= items.length) {
        return;
      }
      results[current] = await mapper(items[current], current);
    }
  });
  await Promise.all(workers);
  return results;
}

async function sameUserDuplicateTest() {
  const adminToken = await adminLogin();
  const token = await register("same");
  const stamp = Date.now();
  const activityID = await createActivity(adminToken, pressureConfig.sameUserStock, `same-${stamp}`);
  const calls = Array.from({ length: pressureConfig.sameUserConcurrency }, (_, i) => postJSON(`${baseURL}/api/orders`, {
    activity_id: activityID,
    request_id: `same-${stamp}-${i}`,
  }, token));
  const results = await Promise.all(calls);
  return summarize(results);
}

async function multiUserStockTest() {
  const adminToken = await adminLogin();
  const stamp = Date.now();
  const activityID = await createActivity(adminToken, pressureConfig.multiStock, `multi-${stamp}`);
  const tokenIndexes = Array.from({ length: pressureConfig.multiUsers }, (_, i) => i);
  const tokens = await mapWithConcurrency(tokenIndexes, pressureConfig.registerConcurrency, (i) => register(`multi${i}`));
  const calls = tokens.map((token, i) => postJSON(`${baseURL}/api/orders`, {
    activity_id: activityID,
    request_id: `multi-${stamp}-${i}`,
  }, token));
  const results = await Promise.all(calls);
  return summarize(results);
}

function summarize(results) {
  const summary = {};
  for (const item of results) {
    summary[item.status] = (summary[item.status] || 0) + 1;
  }
  return { summary, results };
}

async function main() {
  const sameUser = await sameUserDuplicateTest();
  const multiUser = await multiUserStockTest();
  const output = {
    config: pressureConfig,
    sameUserSummary: sameUser.summary,
    multiUserSummary: multiUser.summary,
  };
  fs.writeFileSync(path.join(outDir, "order-pressure-result.json"), JSON.stringify({
    output,
    sameUser,
    multiUser,
  }, null, 2));
  console.log(JSON.stringify(output, null, 2));

  const sameUserAccepted = sameUser.summary["202"] || 0;
  if (sameUserAccepted > 1) {
    throw new Error(`same user accepted ${sameUserAccepted}, want <= 1`);
  }
  const multiUserAccepted = multiUser.summary["202"] || 0;
  if (multiUserAccepted > pressureConfig.multiStock) {
    throw new Error(`multi user accepted ${multiUserAccepted}, want <= stock ${pressureConfig.multiStock}`);
  }
}

if (require.main === module) {
  main().catch((err) => {
    console.error(err);
    process.exit(1);
  });
}

module.exports = {
  parsePressureConfig,
};
