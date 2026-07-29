-- Seed data for manual API testing after GORM AutoMigrate has created tables.
-- The demo user password hash is intentionally not relied on by smoke tests;
-- scripts create fresh users through the public register API.

USE order_lab;

INSERT INTO users (id, username, password_hash, created_at, updated_at)
VALUES
  (1001, 'demo_user', '$2a$10$rDoX6hMNP7zvPBi5cYdZxevYpU1Ai4qQ3EwC.fwV7fckcyoEJFA1W', NOW(), NOW())
ON DUPLICATE KEY UPDATE
  username = VALUES(username),
  updated_at = NOW();

INSERT INTO products (id, name, price, stock, created_at, updated_at)
VALUES
  (1001, 'demo-phone', 299900, 100, NOW(), NOW()),
  (1002, 'demo-headset', 39900, 200, NOW(), NOW())
ON DUPLICATE KEY UPDATE
  name = VALUES(name),
  price = VALUES(price),
  stock = VALUES(stock),
  updated_at = NOW();

INSERT INTO activities (id, product_id, name, price, stock, status, start_at, end_at, created_at, updated_at)
VALUES
  (1001, 1001, 'demo-flash-sale', 269900, 20, 'PUBLISHED', NOW(), DATE_ADD(NOW(), INTERVAL 1 DAY), NOW(), NOW())
ON DUPLICATE KEY UPDATE
  product_id = VALUES(product_id),
  name = VALUES(name),
  price = VALUES(price),
  stock = VALUES(stock),
  status = VALUES(status),
  start_at = VALUES(start_at),
  end_at = VALUES(end_at),
  updated_at = NOW();

INSERT INTO coupons (id, title, threshold, discount, stock, start_at, end_at, created_at, updated_at)
VALUES
  (1001, 'demo-coupon-50', 100000, 5000, 100, NOW(), DATE_ADD(NOW(), INTERVAL 1 DAY), NOW(), NOW())
ON DUPLICATE KEY UPDATE
  title = VALUES(title),
  threshold = VALUES(threshold),
  discount = VALUES(discount),
  stock = VALUES(stock),
  start_at = VALUES(start_at),
  end_at = VALUES(end_at),
  updated_at = NOW();
