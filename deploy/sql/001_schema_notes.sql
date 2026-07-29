-- Go Order Lab database bootstrap notes.
-- GORM AutoMigrate creates and updates application tables in development mode.
-- This file documents the MySQL database and user setup used by the verified VM run.

CREATE DATABASE IF NOT EXISTS order_lab
  CHARACTER SET utf8mb4
  COLLATE utf8mb4_unicode_ci;

CREATE USER IF NOT EXISTS 'order_user'@'localhost' IDENTIFIED BY 'order_pass';
CREATE USER IF NOT EXISTS 'order_user'@'127.0.0.1' IDENTIFIED BY 'order_pass';
CREATE USER IF NOT EXISTS 'order_user'@'%' IDENTIFIED BY 'order_pass';

GRANT ALL PRIVILEGES ON order_lab.* TO 'order_user'@'localhost';
GRANT ALL PRIVILEGES ON order_lab.* TO 'order_user'@'127.0.0.1';
GRANT ALL PRIVILEGES ON order_lab.* TO 'order_user'@'%';
FLUSH PRIVILEGES;
