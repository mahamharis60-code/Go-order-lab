const assert = require("assert");
const { parsePressureConfig } = require("./pressure-order");

const config = parsePressureConfig({
  ORDER_PRESSURE_SAME_USER_CONCURRENCY: "120",
  ORDER_PRESSURE_SAME_USER_STOCK: "9",
  ORDER_PRESSURE_MULTI_USERS: "240",
  ORDER_PRESSURE_MULTI_STOCK: "35",
  ORDER_PRESSURE_REGISTER_CONCURRENCY: "15",
});

assert.strictEqual(config.sameUserConcurrency, 120);
assert.strictEqual(config.sameUserStock, 9);
assert.strictEqual(config.multiUsers, 240);
assert.strictEqual(config.multiStock, 35);
assert.strictEqual(config.registerConcurrency, 15);

const fallback = parsePressureConfig({});

assert.strictEqual(fallback.sameUserConcurrency, 20);
assert.strictEqual(fallback.sameUserStock, 5);
assert.strictEqual(fallback.multiUsers, 10);
assert.strictEqual(fallback.multiStock, 3);
assert.strictEqual(fallback.registerConcurrency, 20);
