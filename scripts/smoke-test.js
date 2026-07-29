const fs = require("fs");
const path = require("path");

const baseURL = process.env.ORDER_BASE_URL || "http://127.0.0.1:8090";
const outDir = process.env.ORDER_SMOKE_OUT || path.join(process.cwd(), "reports", "baseline-run");
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
  fs.writeFileSync(path.join(outDir, name), JSON.stringify({ status: res.status, body }, null, 2));
  return { status: res.status, body, traceID: res.headers.get("x-trace-id") || "" };
}

function postJSON(name, url, body, token = "") {
  const headers = { "Content-Type": "application/json" };
  if (token) headers.Authorization = `Bearer ${token}`;
  return request(name, url, { method: "POST", headers, body: JSON.stringify(body) });
}

function patchJSON(name, url, body, token = "") {
  const headers = { "Content-Type": "application/json" };
  if (token) headers.Authorization = `Bearer ${token}`;
  return request(name, url, { method: "PATCH", headers, body: JSON.stringify(body) });
}

async function main() {
  const stamp = Date.now();
  const adminLogin = await postJSON("order-admin-login.json", `${baseURL}/api/auth/login`, {
    username: adminUsername,
    password: adminPassword,
  });
  const adminToken = adminLogin.body.data.token;

  const register = await postJSON("order-upgraded-register.json", `${baseURL}/api/auth/register`, {
    username: `demo${stamp}`,
    password: "123456",
  });
  const token = register.body.data.token;

  const userCreateProduct = await postJSON("order-rbac-user-create-product.json", `${baseURL}/api/products`, {
    name: `forbidden-phone-${stamp}`,
    price: 99900,
    stock: 1,
  }, token);

  const product = await postJSON("order-upgraded-product.json", `${baseURL}/api/products`, {
    name: `demo-phone-${stamp}`,
    price: 299900,
    stock: 20,
  }, adminToken);

  const activity = await postJSON("order-upgraded-activity.json", `${baseURL}/api/activities`, {
    product_id: product.body.data.id,
    name: `summer-sale-${stamp}`,
    price: 269900,
    stock: 5,
    duration_seconds: 3600,
  }, adminToken);

  const order = await postJSON("order-upgraded-create-order.json", `${baseURL}/api/orders`, {
    activity_id: activity.body.data.id,
    request_id: `req-${stamp}`,
  }, token);

  await new Promise((resolve) => setTimeout(resolve, 700));

  const orderNo = order.body.data.order_no;
  const list = await request("order-upgraded-list-orders.json", `${baseURL}/api/orders`, {
    headers: { Authorization: `Bearer ${token}` },
  });
  const adminOverview = await request("order-admin-overview.json", `${baseURL}/api/admin/overview`, {
    headers: { Authorization: `Bearer ${adminToken}` },
  });
  const adminOrders = await request("order-admin-orders.json", `${baseURL}/api/admin/orders?status=WAIT_PAY&activity_id=${activity.body.data.id}&limit=5`, {
    headers: { Authorization: `Bearer ${adminToken}` },
  });
  const payment = await postJSON("order-upgraded-payment.json", `${baseURL}/api/payments/callback`, {
    order_no: orderNo,
    transaction_no: `pay-${stamp}`,
    status: "SUCCESS",
  });
  const duplicatePayment = await postJSON("order-upgraded-payment-duplicate.json", `${baseURL}/api/payments/callback`, {
    order_no: orderNo,
    transaction_no: `pay-${stamp}`,
    status: "SUCCESS",
  });
  const duplicateOrder = await postJSON("order-upgraded-duplicate-order.json", `${baseURL}/api/orders`, {
    activity_id: activity.body.data.id,
    request_id: `req-dup-${stamp}`,
  }, token);
  const offlineProduct = await postJSON("order-compensation-offline-product.json", `${baseURL}/api/products`, {
    name: `offline-phone-${stamp}`,
    price: 199900,
    stock: 5,
  }, adminToken);
  const offlineActivity = await postJSON("order-compensation-offline-activity.json", `${baseURL}/api/activities`, {
    product_id: offlineProduct.body.data.id,
    name: `offline-sale-${stamp}`,
    price: 179900,
    stock: 2,
    duration_seconds: 3600,
  }, adminToken);
  const offlineStatus = await patchJSON("order-compensation-offline-status.json", `${baseURL}/api/activities/${offlineActivity.body.data.id}/status`, {
    status: "OFFLINE",
  }, adminToken);
  const offlineOrder = await postJSON("order-compensation-offline-order.json", `${baseURL}/api/orders`, {
    activity_id: offlineActivity.body.data.id,
    request_id: `offline-req-${stamp}`,
  }, token);
  const logs = await request("order-upgraded-logs.json", `${baseURL}/api/order-logs`, {
    headers: { Authorization: `Bearer ${adminToken}` },
  });

  const address = await postJSON("order-full-address.json", `${baseURL}/api/addresses`, {
    receiver: "demo-user",
    phone: "13800000000",
    province: "湖北",
    city: "武汉",
    detail: "光谷软件园 A 座",
    is_default: true,
  }, token);
  const cartItem = await postJSON("order-full-cart-add.json", `${baseURL}/api/cart/items`, {
    product_id: product.body.data.id,
    quantity: 2,
  }, token);
  const cartList = await request("order-full-cart-list.json", `${baseURL}/api/cart/items`, {
    headers: { Authorization: `Bearer ${token}` },
  });
  const coupon = await postJSON("order-full-coupon-create.json", `${baseURL}/api/coupons`, {
    title: `demo-coupon-${stamp}`,
    threshold: 100000,
    discount: 5000,
    stock: 10,
    duration_seconds: 3600,
  }, adminToken);
  const userCoupon = await postJSON("order-full-coupon-claim.json", `${baseURL}/api/coupons/${coupon.body.data.id}/claim`, {}, token);
  const checkout = await postJSON("order-full-checkout.json", `${baseURL}/api/orders/checkout`, {
    address_id: address.body.data.id,
    user_coupon_id: userCoupon.body.data.id,
  }, token);
  const checkoutPayment = await postJSON("order-full-checkout-payment.json", `${baseURL}/api/payments/callback`, {
    order_no: checkout.body.data.order_no,
    transaction_no: `cart-pay-${stamp}`,
    status: "SUCCESS",
  });
  const timeoutCartItem = await postJSON("order-full-timeout-cart-add.json", `${baseURL}/api/cart/items`, {
    product_id: product.body.data.id,
    quantity: 1,
  }, token);
  const timeoutCheckout = await postJSON("order-full-timeout-checkout.json", `${baseURL}/api/orders/checkout`, {
    address_id: address.body.data.id,
  }, token);
  const compensateOrders = await postJSON("order-compensation-run.json", `${baseURL}/api/ops/compensate`, {
    queued_timeout_seconds: 30,
    pay_timeout_seconds: 0,
  }, adminToken);
  const expireOrders = await postJSON("order-full-expire-orders.json", `${baseURL}/api/orders/expire`, {
    timeout_seconds: 0,
  }, adminToken);

  console.log(JSON.stringify({
    adminLogin: adminLogin.status,
    register: register.status,
    userCreateProduct: userCreateProduct.status,
    product: product.status,
    activity: activity.status,
    order: order.status,
    orderTraceID: order.traceID,
    registerTraceID: register.traceID,
    list: list.status,
    adminOverview: adminOverview.status,
    adminOrders: adminOrders.status,
    adminOrderTotal: adminOrders.body.data.total,
    payment: payment.status,
    duplicatePayment: duplicatePayment.status,
    duplicateOrder: duplicateOrder.status,
    offlineStatus: offlineStatus.status,
    offlineOrder: offlineOrder.status,
    logs: logs.status,
    address: address.status,
    cartItem: cartItem.status,
    cartList: cartList.status,
    coupon: coupon.status,
    userCoupon: userCoupon.status,
    checkout: checkout.status,
    checkoutPayment: checkoutPayment.status,
    timeoutCartItem: timeoutCartItem.status,
    timeoutCheckout: timeoutCheckout.status,
    compensateOrders: compensateOrders.status,
    compensateClosedOrders: compensateOrders.body.data.closed_orders,
    compensateRequeuedOrders: compensateOrders.body.data.requeued_orders,
    compensateEndedActivities: compensateOrders.body.data.ended_activities,
    expireOrders: expireOrders.status,
    expiredCount: expireOrders.body.data.expired_count,
    orderNo,
    checkoutOrderNo: checkout.body.data.order_no,
    timeoutOrderNo: timeoutCheckout.body.data.order_no,
  }, null, 2));
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
