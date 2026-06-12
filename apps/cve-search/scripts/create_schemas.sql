-- Run as postgres superuser before all service migrations
-- This script creates all required schemas and extensions.

-- Schemas per service
CREATE SCHEMA IF NOT EXISTS auth;
CREATE SCHEMA IF NOT EXISTS scan;
CREATE SCHEMA IF NOT EXISTS asset;
CREATE SCHEMA IF NOT EXISTS cve;
CREATE SCHEMA IF NOT EXISTS agent;
CREATE SCHEMA IF NOT EXISTS schedule;
CREATE SCHEMA IF NOT EXISTS report;
CREATE SCHEMA IF NOT EXISTS notification;

-- Extensions (require superuser / rds_superuser)
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";
CREATE EXTENSION IF NOT EXISTS "citext";
CREATE EXTENSION IF NOT EXISTS "vector";  -- pgvector for cve.cves embedding column

-- Grant schema usage to application user (replace 'ovs_app' with actual user)
-- GRANT USAGE ON SCHEMA auth, scan, asset, cve, agent, schedule, report, notification
--   TO ovs_app;
-- GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA auth TO ovs_app;
-- (repeat for each schema)
