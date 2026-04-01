-- ============================================================
-- DistroCommerce — Postgres initialization script
-- Runs once on first container start (docker-entrypoint-initdb.d)
-- Creates one database + user per service.
-- In prod each service gets its own RDS instance instead.
-- ============================================================

-- identity-sv
CREATE USER identity_svc WITH PASSWORD 'secret';
CREATE DATABASE identity_db OWNER identity_svc;

-- user-sv
CREATE USER user_svc WITH PASSWORD 'secret';
CREATE DATABASE user_db OWNER user_svc;

-- order-sv
CREATE USER order_svc WITH PASSWORD 'secret';
CREATE DATABASE order_db OWNER order_svc;

-- payment-sv
CREATE USER payment_svc WITH PASSWORD 'secret';
CREATE DATABASE payment_db OWNER payment_svc;

-- inventory-sv
CREATE USER inventory_svc WITH PASSWORD 'secret';
CREATE DATABASE inventory_db OWNER inventory_svc;
