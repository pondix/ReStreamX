CREATE DATABASE IF NOT EXISTS demo;
CREATE TABLE IF NOT EXISTS demo.accounts (
  id INT PRIMARY KEY,
  balance INT NOT NULL,
  updated_at TIMESTAMP NOT NULL
);
CREATE TABLE IF NOT EXISTS demo.orders (
  id INT PRIMARY KEY,
  balance INT NOT NULL,
  updated_at TIMESTAMP NOT NULL
);
CREATE DATABASE IF NOT EXISTS rlr_meta;
CREATE TABLE IF NOT EXISTS rlr_meta.applied_segments (
  range_id VARCHAR(128) NOT NULL,
  epoch BIGINT NOT NULL,
  txn_id VARCHAR(64) NOT NULL,
  commit_index BIGINT NOT NULL,
  applied_at TIMESTAMP NOT NULL,
  PRIMARY KEY (range_id, epoch, txn_id)
);
CREATE USER IF NOT EXISTS 'restreamx_router'@'%' IDENTIFIED BY 'router';
GRANT INSERT,UPDATE,DELETE,SELECT ON demo.* TO 'restreamx_router'@'%';
CREATE USER IF NOT EXISTS 'restreamx_apply'@'%' IDENTIFIED BY 'apply';
GRANT INSERT,UPDATE,DELETE,SELECT ON demo.* TO 'restreamx_apply'@'%';
GRANT INSERT,UPDATE ON rlr_meta.* TO 'restreamx_apply'@'%';
FLUSH PRIVILEGES;
